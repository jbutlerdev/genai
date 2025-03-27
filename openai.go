package genai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/jbutlerdev/genai/tools"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

type OpenAIClient struct {
	client openai.Client
	log    logr.Logger
	Tools  []*tools.Tool
}

func NewOpenAIClient(provider *Provider) (*OpenAIClient, error) {
	options := []option.RequestOption{
		option.WithAPIKey(provider.APIKey),
	}
	if provider.BaseURL != "" {
		provider.Log.Info("setting base URL", "baseURL", provider.BaseURL)
		options = append(options, option.WithBaseURL(provider.BaseURL))
	}
	client := openai.NewClient(options...)

	return &OpenAIClient{
		client: client,
		log:    provider.Log,
		Tools:  make([]*tools.Tool, 0),
	}, nil
}

func (c *OpenAIClient) Models() []string {
	// Default models to return as fallback
	defaultModels := []string{
		"gpt-4",
		"gpt-4-turbo-preview",
		"gpt-3.5-turbo",
	}

	// List all models
	var allModels []string
	pager := c.client.Models.ListAutoPaging(context.Background())
	for pager.Next() {
		model := pager.Current()
		allModels = append(allModels, model.ID)
	}
	if pager.Err() != nil {
		c.log.Error(pager.Err(), "failed to list models")
		return defaultModels
	}

	if len(allModels) == 0 {
		c.log.Error(fmt.Errorf("no models found"), "no models found")
		return defaultModels
	}

	return allModels
}

func (c *OpenAIClient) Generate(ctx context.Context, modelName string, prompt string) (string, error) {
	params := openai.ChatCompletionNewParams{
		Model: modelName,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	}

	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return resp.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) ConvertToolToFunction(tool *tools.Tool) openai.FunctionDefinition {
	params := make(map[string]interface{})
	required := make([]string, 0)
	properties := make(map[string]interface{})

	for _, param := range tool.Parameters {
		properties[param.Name] = map[string]string{
			"type":        param.Type,
			"description": param.Description,
		}
		if param.Required {
			required = append(required, param.Name)
		}
	}

	params["type"] = "object"
	params["properties"] = properties
	params["required"] = required

	return openai.FunctionDefinition{
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  shared.FunctionParameters(params),
	}
}

func (c *OpenAIClient) Chat(ctx context.Context, model string, systemPrompt string, chat *Chat, provider *Provider, messages []openai.ChatCompletionMessage) error {
	if systemPrompt != "" && len(messages) == 0 {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	for {
		select {
		case newMessage := <-chat.Send:
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    "user",
				Content: newMessage,
			})
			chat.Logger.Info("Sending message to OpenAI", "content", newMessage)

			// Process this message and any subsequent tool calls
			if err := c.processOpenAIMessage(ctx, model, chat, provider, messages); err != nil {
				chat.Logger.Error(err, "Failed to process message")
			}

		case <-chat.Done:
			return nil
		}
		chat.GenerationComplete <- true
	}
}

// processOpenAIMessage handles a message (user input or tool response) and any subsequent tool calls
func (c *OpenAIClient) processOpenAIMessage(ctx context.Context, model string, chat *Chat, provider *Provider, messages []openai.ChatCompletionMessage) error {
	// Create parameters for OpenAI API
	var paramMessages []openai.ChatCompletionMessageParamUnion

	// Map to track tool_call_id for tool messages
	toolCallIDs := make(map[int]string)

	for i, msg := range messages {
		switch msg.Role {
		case "user":
			paramMessages = append(paramMessages, openai.UserMessage(msg.Content))
		case "assistant":
			// If there are tool calls, we need to preserve them in the message history
			if len(msg.ToolCalls) > 0 {
				// We need to use the raw ToParam() method to preserve tool calls
				paramMessages = append(paramMessages, msg.ToParam())

				// Save tool call IDs for subsequent tool responses
				for j, toolCall := range msg.ToolCalls {
					if i+1+j < len(messages) && messages[i+1+j].Role == "tool" {
						toolCallIDs[i+1+j] = toolCall.ID
					}
				}
			} else {
				paramMessages = append(paramMessages, openai.AssistantMessage(msg.Content))
			}
		case "system":
			paramMessages = append(paramMessages, openai.SystemMessage(msg.Content))
		case "tool":
			// For tool messages, we need to use ToolMessage with the proper tool_call_id
			if id, ok := toolCallIDs[i]; ok {
				paramMessages = append(paramMessages, openai.ToolMessage(msg.Content, id))
			} else {
				// Fallback - try to find a matching tool call in the previous message
				foundID := ""
				if i > 0 && messages[i-1].Role == "assistant" && len(messages[i-1].ToolCalls) > 0 {
					// Just use the first tool call ID as a fallback
					foundID = messages[i-1].ToolCalls[0].ID
					chat.Logger.Info("Using fallback tool call ID from previous message", "index", i, "id", foundID)
				}

				if foundID != "" {
					paramMessages = append(paramMessages, openai.ToolMessage(msg.Content, foundID))
				} else {
					// Last resort fallback - not ideal but better than nothing
					chat.Logger.Info("No tool call ID found, using AssistantMessage as fallback", "index", i)
					paramMessages = append(paramMessages, openai.AssistantMessage(msg.Content))
				}
			}
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: paramMessages,
	}

	// Add tools if any
	if len(c.Tools) > 0 {
		var tools []openai.ChatCompletionToolParam
		for _, tool := range c.Tools {
			fn := c.ConvertToolToFunction(tool)
			tools = append(tools, openai.ChatCompletionToolParam{
				Type: "function",
				Function: shared.FunctionDefinitionParam{
					Name:        fn.Name,
					Parameters:  fn.Parameters,
					Description: openai.String(fn.Description),
				},
			})
		}
		params.Tools = tools
	}

	jsonParams, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		chat.Logger.Error(err, "Failed to marshal params for logging")
	} else {
		chat.Logger.Info("Params", "content", string(jsonParams))
	}

	// Get response
	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response choices returned")
	}

	choice := resp.Choices[0]
	/*
		jsonMessage, err := json.MarshalIndent(choice.Message, "", "  ")
		if err != nil {
			chat.Logger.Error(err, "Failed to marshal message for logging")
		} else {
			chat.Logger.Info("Raw response", "content", string(jsonMessage))
		}
	*/

	// Handle tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		// Save the assistant's response with tool calls
		assistantMsg := openai.ChatCompletionMessage{
			Role:      "assistant",
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		}
		messages = append(messages, assistantMsg)

		toolCallsProcessed := false
		var toolResponses []openai.ChatCompletionMessage

		// Process each tool call
		for i, toolCall := range choice.Message.ToolCalls {
			if toolCall.Type == "function" {
				chat.Logger.Info("Handling function call", "index", i, "id", toolCall.ID, "name", toolCall.Function.Name)

				// Parse arguments to map
				var argsMap map[string]interface{}
				err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap)
				if err != nil {
					chat.Logger.Error(err, "Failed to parse tool arguments")
					continue
				}

				// Execute the tool
				result, err := provider.RunTool(toolCall.Function.Name, argsMap)
				if err != nil {
					chat.Logger.Error(err, "Failed to run tool")
					continue
				}

				resultStr := fmt.Sprintf("%v", result)

				// Add the tool result as a custom message - we'll handle this specially when converting to params
				toolResponse := openai.ChatCompletionMessage{
					Role:    "tool",
					Content: resultStr,
					// We'll use the toolCall.ID when converting to a parameter
				}
				toolResponses = append(toolResponses, toolResponse)
				// Also track the ID for this response to use later
				nextIndex := len(messages) + len(toolResponses) - 1
				toolCallIDs[nextIndex] = toolCall.ID
				toolCallsProcessed = true
			}
		}

		// Add all tool responses to the conversation history
		messages = append(messages, toolResponses...)

		// If we processed any tool calls, recursively process the updated messages
		if toolCallsProcessed {
			return c.processOpenAIMessage(ctx, model, chat, provider, messages)
		}
	}

	// Handle text response
	response := choice.Message.Content
	chat.Logger.Info("Handling text", "content", response)
	// Send the response to the chat
	chat.Recv <- response
	return nil
}
