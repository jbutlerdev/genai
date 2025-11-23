package genai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/jbutlerdev/genai/tools"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"github.com/tiktoken-go/tokenizer"
)

const (
	openaiTimeout = 1 * time.Hour
)

type OpenAIClient struct {
	client  openai.Client
	log     logr.Logger
	Tools   []*tools.Tool
	enc     tokenizer.Codec
	model   string
	baseURL string
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

	c, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		provider.Log.Error(err, "Failed to create tokenizer")
		return nil, err
	}
	
	// Set default model for embeddings
	model := "text-embedding-3-small"
	// If provider has a model specified, use it
	if provider.Model != nil && provider.Model.openAIModel != "" {
		model = provider.Model.openAIModel
	}
	
	return &OpenAIClient{
		client:  client,
		log:     provider.Log,
		Tools:   make([]*tools.Tool, 0),
		enc:     c,
		model:   model,
		baseURL: provider.BaseURL,
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

func newParams(model string, messages []openai.ChatCompletionMessageParamUnion, params map[string]any) openai.ChatCompletionNewParams {
	messageParams := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: messages,
	}
	for k, v := range params {
		switch k {
		case RepeatPenalty:
			penalty, ok := v.(float64)
			if !ok {
				penalty = 1.0
			}
			messageParams.FrequencyPenalty = param.Opt[float64]{Value: penalty}
		case Temperature:
			temperature, ok := v.(float64)
			if !ok {
				temperature = 1.0
			}
			messageParams.Temperature = param.Opt[float64]{Value: temperature}
		case Seed:
			seed, ok := v.(int64)
			if !ok {
				seed = 0
			}
			messageParams.Seed = param.Opt[int64]{Value: seed}
		case NumPredict:
			numPredict, ok := v.(int)
			if !ok {
				// Try to convert from float64 (JSON numbers often come as float64)
				if floatVal, ok := v.(float64); ok {
					numPredict = int(floatVal)
				} else {
					// Don't set if not a valid number
					continue
				}
			}
			// Set both MaxTokens and MaxCompletionTokens for compatibility
			// llama.cpp OpenAI API should respect max_tokens
			messageParams.MaxTokens = param.Opt[int64]{Value: int64(numPredict)}
			messageParams.MaxCompletionTokens = param.Opt[int64]{Value: int64(numPredict)}
		case TopP:
			topP, ok := v.(float64)
			if !ok {
				topP = 1.0
			}
			messageParams.TopP = param.Opt[float64]{Value: topP}
		}
	}
	return messageParams
}

func (c *OpenAIClient) Generate(ctx context.Context, modelOptions ModelOptions, systemPrompt string, prompt string) (string, error) {
	messages := []openai.ChatCompletionMessageParamUnion{}
	if systemPrompt != "" {
		messages = append(messages, openai.SystemMessage(systemPrompt))
	}
	messages = append(messages, openai.UserMessage(prompt))
	params := newParams(modelOptions.ModelName, messages, modelOptions.Parameters)

	generateContext, cancel := context.WithTimeout(ctx, openaiTimeout)
	defer cancel()
	resp, err := c.client.Chat.Completions.New(generateContext, params)
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
		switch param.Type {
		case "stringArray":
			properties[param.Name] = map[string]interface{}{
				"type":        "array",
				"description": param.Description,
				"items": map[string]interface{}{
					"type": "string",
				},
			}
		default:
			properties[param.Name] = map[string]string{
				"type":        param.Type,
				"description": param.Description,
			}
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

func (c *OpenAIClient) Chat(ctx context.Context, m *Model, chat *Chat, messages []openai.ChatCompletionMessage) error {
	if m.SystemPrompt != "" && len(messages) == 0 {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    "system",
			Content: m.SystemPrompt,
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
			if err := c.processOpenAIMessage(ctx, m, chat, messages); err != nil {
				chat.Logger.Error(err, "Failed to process message")
			}

		case <-chat.Done:
			return nil
		}
		chat.GenerationComplete <- true
	}
}

// transform messages array into string
func messagesToString(messages []openai.ChatCompletionMessage, includeSystem bool) string {
	content := ""
	for _, msg := range messages {
		if !includeSystem && msg.Role == "system" {
			continue
		}
		content += fmt.Sprintf("{\"Role\": \"%s\", \"content\": \"%s\"}", msg.Role, msg.Content)
		// need to add other fields
	}
	return content
}

// handle message size
// if context grows larger than model NumCtx then shrink it
func handleContextLength(m *Model, messages []openai.ChatCompletionMessage, c tokenizer.Codec) ([]openai.ChatCompletionMessage, error) {
	maxContext, ok := m.Parameters[NumCtx].(int)
	if !ok {
		return nil, errors.New("failed to parse num_ctx for model")
	}
	content := messagesToString(messages, true)
	contextSize, err := c.Count(content)
	if err != nil {
		return nil, err
	}
	if contextSize > maxContext {
		m.Logger.Info("context length is larger than NumCtx, compacting...", "length", strconv.Itoa(contextSize))
		return compact(m, messages)
	}
	return messages, nil
}

// compact messages
func compact(m *Model, messages []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, error) {
	prompt := "Compact this conversation into 5000 words or less. Do not include any word counts or summarizing. Just return the summarized content.\n"
	prompt += messagesToString(messages, false)
	modelOptions := ModelOptions{
		ModelName:    m.openAIModel,
		SystemPrompt: m.SystemPrompt,
		Parameters:   m.Parameters,
		MaxTurns:     m.MaxTurns,
	}
	response, err := m.generate(prompt, modelOptions)
	if err != nil {
		return nil, err
	}
	responseMessages := []openai.ChatCompletionMessage{messages[0]}
	if messages[0].Role == "system" {
		responseMessages = append(responseMessages, messages[1])
	}
	compactMessage := openai.ChatCompletionMessage{
		Role:    "user",
		Content: response,
	}
	responseMessages = append(responseMessages, compactMessage)
	return responseMessages, nil
}

// executeToolCall executes a single tool call with its own 5-minute timeout context
func (c *OpenAIClient) executeToolCall(ctx context.Context, m *Model, chat *Chat, toolCall openai.ChatCompletionMessageToolCall) (string, error) {
	// Create a context with 5-minute timeout for this specific tool call
	toolCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	funcJson, err := json.MarshalIndent(toolCall.Function, "", "  ")
	if err != nil {
		chat.Logger.Error(err, "Failed to marshal tool call arguments", "tool", toolCall.Function.Name)
	}
	chat.Logger.Info("Handling function call", "name", toolCall.Function.Name, "content", string(funcJson))

	// Parse arguments to map
	var argsMap map[string]interface{}
	err = json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap)
	if err != nil {
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Create a channel to receive the result
	type toolResult struct {
		result interface{}
		err    error
	}
	resultChan := make(chan toolResult, 1)

	// Execute the tool in a goroutine
	go func() {
		result, err := m.Provider.RunTool(toolCall.Function.Name, argsMap)
		resultChan <- toolResult{result: result, err: err}
	}()

	// Wait for either the tool to complete or the context to timeout
	select {
	case <-toolCtx.Done():
		// Context timed out
		return "", fmt.Errorf("tool call %s timed out after 5 minutes: %w", toolCall.Function.Name, toolCtx.Err())
	case res := <-resultChan:
		// Tool completed
		if res.err != nil {
			return "", fmt.Errorf("tool execution failed: %w", res.err)
		}
		return fmt.Sprintf("%v", res.result), nil
	}
}

// processToolCalls handles executing multiple tool calls
func (c *OpenAIClient) processToolCalls(ctx context.Context, m *Model, chat *Chat, toolCalls []openai.ChatCompletionMessageToolCall, messages []openai.ChatCompletionMessage, toolCallIDs map[int]string) (bool, []openai.ChatCompletionMessage, error) {
	toolCallsProcessed := false
	var toolResponses []openai.ChatCompletionMessage

	// Process each tool call
	for _, toolCall := range toolCalls {
		if toolCall.Type == "function" {
			// Execute the tool with its own timeout
			resultStr, err := c.executeToolCall(ctx, m, chat, toolCall)
			if err != nil {
				chat.Logger.Error(err, "Failed to execute tool call", "tool", toolCall.Function.Name)
			}

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

	return toolCallsProcessed, toolResponses, nil
}

func messagesToParamUnion(chat *Chat, messages []openai.ChatCompletionMessage, toolCallIDs map[int]string) []openai.ChatCompletionMessageParamUnion {
	// Create parameters for OpenAI API
	var paramMessages []openai.ChatCompletionMessageParamUnion
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
	return paramMessages
}

func (c *OpenAIClient) handleTurns(ctx context.Context, m *Model, chat *Chat, messages openai.ChatCompletionNewParams) (bool, error) {
	chat.Turns++
	if m.MaxTurns > 0 && chat.Turns > m.MaxTurns {
		processContext, cancel := context.WithTimeout(ctx, openaiTimeout)
		defer cancel()
		resp, err := c.client.Chat.Completions.New(processContext, messages)
		if err != nil {
			return true, fmt.Errorf("failed to generate final chat message: %w", err)
		}
		return true, c.handleResponse(ctx, resp, m, chat, nil, nil)
	}
	return false, nil
}

func (c *OpenAIClient) handleResponse(ctx context.Context, resp *openai.ChatCompletion, m *Model, chat *Chat, messages []openai.ChatCompletionMessage, toolCallIDs map[int]string) error {
	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response choices returned")
	}

	choice := resp.Choices[0]

	// Handle tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		// Save the assistant's response with tool calls
		assistantMsg := openai.ChatCompletionMessage{
			Role:      "assistant",
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		}
		messages = append(messages, assistantMsg)

		// Process tool calls with the new function
		toolCallsProcessed, toolResponses, err := c.processToolCalls(ctx, m, chat, choice.Message.ToolCalls, messages, toolCallIDs)
		if err != nil {
			return fmt.Errorf("failed to process tool calls: %w", err)
		}

		// Add all tool responses to the conversation history
		messages = append(messages, toolResponses...)

		// If we processed any tool calls, recursively process the updated messages
		if toolCallsProcessed {
			return c.processOpenAIMessage(ctx, m, chat, messages)
		}
	}

	// Handle text response
	response := choice.Message.Content
	chat.Logger.Info("Handling text", "content", response)
	
	// Check if the response contains invalid tool call markers
	if strings.Contains(response, "<tool_call>") {
		chat.Logger.Info("Detected invalid tool call in response")
		
		// Add the assistant's invalid message to history (but don't send to user)
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: response,
		})
		
		// Add an error message about invalid tool call
		invalidToolCallMsg := "Error: Invalid tool call format detected. Please use the proper tool calling mechanism instead of embedding tool calls in text."
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    "user",
			Content: invalidToolCallMsg,
		})
		
		// Process the error message to get a corrected response
		return c.processOpenAIMessage(ctx, m, chat, messages)
	}
	
	// Send the response to the chat
	chat.Recv <- response
	return nil
}

// processOpenAIMessage handles a message (user input or tool response) and any subsequent tool calls
func (c *OpenAIClient) processOpenAIMessage(ctx context.Context, m *Model, chat *Chat, messages []openai.ChatCompletionMessage) error {
	// Map to track tool_call_id for tool messages
	toolCallIDs := make(map[int]string)

	// validate context length
	var err error
	if m.Parameters == nil {
		return errors.New("nil Parameters attached to model")
	}
	messages, err = handleContextLength(m, messages, c.enc)
	if err != nil {
		return err
	}

	paramMessages := messagesToParamUnion(chat, messages, toolCallIDs)

	params := openai.ChatCompletionNewParams{
		Model:    m.openAIModel,
		Messages: paramMessages,
	}

	done, err := c.handleTurns(ctx, m, chat, params)
	if err != nil {
		chat.Logger.Error(err, "error occurred when calculating max turns")
	}
	if done {
		return err
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
					Description: param.NewOpt(fn.Description),
				},
			})
		}
		params.Tools = tools
	}

	// Get response
	processContext, cancel := context.WithTimeout(ctx, openaiTimeout)
	defer cancel()
	resp, err := c.client.Chat.Completions.New(processContext, params)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return c.handleResponse(ctx, resp, m, chat, messages, toolCallIDs)
}

// GenerateEmbedding generates an embedding for a single text input using OpenAI's embedding API
func (c *OpenAIClient) GenerateEmbedding(ctx context.Context, text string, model string) ([]float32, error) {
	// Use the provided model parameter or fallback to text-embedding-3-small
	if model == "" {
		model = "text-embedding-3-small"
	}

	// Log the model being used for debugging
	c.log.Info("Generating embedding with model", "model", model, "baseURL", c.baseURL)

	params := openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: param.NewOpt(text),
		},
		Model: openai.EmbeddingModel(model),
	}

	resp, err := c.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	// Convert []float64 to []float32
	embedding := make([]float32, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// GenerateEmbeddings generates embeddings for multiple text inputs using OpenAI's embedding API
func (c *OpenAIClient) GenerateEmbeddings(ctx context.Context, texts []string, model string) ([][]float32, error) {
	// Use the provided model parameter or fallback to text-embedding-3-small
	if model == "" {
		model = "text-embedding-3-small"
	}

	// Log the model being used for debugging
	c.log.Info("Generating embeddings with model", "model", model, "baseURL", c.baseURL)

	params := openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		},
		Model: openai.EmbeddingModel(model),
	}

	resp, err := c.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, embedding := range resp.Data {
		// Convert []float64 to []float32
		embeddings[i] = make([]float32, len(embedding.Embedding))
		for j, v := range embedding.Embedding {
			embeddings[i][j] = float32(v)
		}
	}

	return embeddings, nil
}

// GenerateEmbeddings generates embeddings for multiple text inputs using OpenAI's embedding API
