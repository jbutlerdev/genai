package genai

import (
	"context"
	"fmt"

	gemini "github.com/google/generative-ai-go/genai"
	ollama "github.com/ollama/ollama/api"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type Client struct {
	ctx      context.Context
	provider string
	Gemini   *gemini.Client
	Ollama   *ollama.Client
}

func NewClient(provider *Provider) (*Client, error) {
	ctx := context.Background()
	client := &Client{
		ctx:      ctx,
		provider: provider.Provider,
	}
	switch provider.Provider {
	case GEMINI:
		g, err := gemini.NewClient(ctx, option.WithAPIKey(provider.APIKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %v", err)
		}
		client.Gemini = g
	case OLLAMA:
		client.Ollama = NewOllamaClient(provider.BaseURL)
	}
	return client, nil
}

func (c *Client) Models() []string {
	switch c.provider {
	case GEMINI:
		return c.getGeminiModels()
	case OLLAMA:
		return c.getOllamaModels()
	}
	return []string{}
}

func (c *Client) getGeminiModels() []string {
	iter := c.Gemini.ListModels(c.ctx)
	var geminiModels []string
	for {
		model, err := iter.Next()
		if err == iterator.Done {
			break
		}
		geminiModels = append(geminiModels, model.Name)
	}
	return geminiModels
}

func (c *Client) getOllamaModels() []string {
	models, err := c.Ollama.List(c.ctx)
	if err != nil {
		fmt.Printf("failed to get Ollama models: %v", err)
		return []string{}
	}
	var ollamaModels []string
	for _, model := range models.Models {
		ollamaModels = append(ollamaModels, model.Name)
	}
	return ollamaModels
}
