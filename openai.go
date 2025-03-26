package genai

import (
	"context"
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
}

func NewOpenAIClient(provider *Provider) (*OpenAIClient, error) {
	client := openai.NewClient(option.WithAPIKey(provider.APIKey))
	if provider.BaseURL != "" {
		client.Options = append(client.Options, option.WithBaseURL(provider.BaseURL))
	}

	return &OpenAIClient{
		client: client,
		log:    provider.Log,
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

func (c *OpenAIClient) Chat(ctx context.Context, modelName string, messages []openai.ChatCompletionMessage) (string, error) {
	var paramMessages []openai.ChatCompletionMessageParamUnion
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			paramMessages = append(paramMessages, openai.UserMessage(msg.Content))
		case "assistant":
			paramMessages = append(paramMessages, openai.AssistantMessage(msg.Content))
		case "system":
			paramMessages = append(paramMessages, openai.SystemMessage(msg.Content))
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    modelName,
		Messages: paramMessages,
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

func (c *OpenAIClient) StreamChat(ctx context.Context, modelName string, messages []openai.ChatCompletionMessage, send chan<- string, recv <-chan string, done chan<- bool) {
	var paramMessages []openai.ChatCompletionMessageParamUnion
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			paramMessages = append(paramMessages, openai.UserMessage(msg.Content))
		case "assistant":
			paramMessages = append(paramMessages, openai.AssistantMessage(msg.Content))
		case "system":
			paramMessages = append(paramMessages, openai.SystemMessage(msg.Content))
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    modelName,
		Messages: paramMessages,
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, params)
	if stream.Err() != nil {
		c.log.Error(stream.Err(), "failed to create chat completion stream")
		done <- true
		return
	}

	go func() {
		defer func() { done <- true }()
		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					send <- content
				}
			}
		}
		if stream.Err() != nil {
			c.log.Error(stream.Err(), "stream error")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-recv:
			if !ok {
				return
			}
			paramMessages = append(paramMessages, openai.UserMessage(msg))
		}
	}
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

func (c *OpenAIClient) ConvertFunctionsToTools(functions []openai.FunctionDefinition) []*tools.Tool {
	var result []*tools.Tool
	for _, fn := range functions {
		params := make([]tools.Parameter, 0)
		if props, ok := fn.Parameters["properties"].(map[string]interface{}); ok {
			required := make(map[string]bool)
			if req, ok := fn.Parameters["required"].([]string); ok {
				for _, r := range req {
					required[r] = true
				}
			}

			for name, prop := range props {
				if propMap, ok := prop.(map[string]interface{}); ok {
					param := tools.Parameter{
						Name:        name,
						Type:        propMap["type"].(string),
						Description: propMap["description"].(string),
						Required:    required[name],
					}
					params = append(params, param)
				}
			}
		}

		tool := &tools.Tool{
			Name:        fn.Name,
			Description: fn.Description,
			Parameters:  params,
		}
		result = append(result, tool)
	}
	return result
}
