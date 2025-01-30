package genai

import (
	"context"
	"log"

	"genai/tools"
)

const (
	// debug option for verbose output
	DEBUG = true

	GEMINI    = "gemini"
	ANTHROPIC = "anthropic"
	OPENAI    = "openai"
	OLLAMA    = "ollama"
)

type Provider struct {
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
	Client   *Client
	Model    *Model
}

type Chat struct {
	ctx  context.Context
	Send chan string
	Recv chan string
	Done chan bool
}

func NewProvider(provider string, apiKey string) (*Provider, error) {
	p := &Provider{Provider: provider, APIKey: apiKey}
	client, err := NewClient(p)
	if err != nil {
		return nil, err
	}
	p.Client = client
	return p, nil
}

func (p *Provider) Models() []string {
	return p.Client.Models()
}

func (p *Provider) Chat(modelName string, toolsToUse []*tools.Tool) *Chat {
	chat := &Chat{
		ctx:  p.Client.ctx,
		Send: make(chan string),
		Recv: make(chan string),
		Done: make(chan bool),
	}
	model := NewModel(p, modelName)
	for _, tool := range toolsToUse {
		model.AddTool(tool)
	}
	go model.chat(chat.ctx, chat)

	return chat
}

func (p *Provider) Generate(modelName string, prompt string) (string, error) {
	model := NewModel(p, modelName)
	return model.generate(prompt)
}

func (p *Provider) RunTool(toolName string, args map[string]any) (any, error) {
	tool, err := tools.GetTool(toolName)
	if err != nil {
		return nil, err
	}
	for key, value := range tool.Options {
		args[key] = value
	}
	if DEBUG {
		log.Println("Running tool", toolName, args)
	}
	switch p.Provider {
	case GEMINI:
		return tools.RunGeminiTool(toolName, args)
	}
	return nil, nil
}
