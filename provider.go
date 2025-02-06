package genai

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/jbutlerdev/genai/tools"
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
	Log      logr.Logger
}

type Chat struct {
	ctx                context.Context
	Send               chan string
	Recv               chan string
	GenerationComplete chan bool
	Done               chan bool
	Logger             logr.Logger
}

func NewProvider(provider string, apiKey string) (*Provider, error) {
	l := logr.Discard()
	p := &Provider{Provider: provider, APIKey: apiKey, Log: l}
	client, err := NewClient(p)
	if err != nil {
		return nil, err
	}
	p.Client = client
	return p, nil
}

func NewProviderWithLog(provider string, apiKey string, log logr.Logger) (*Provider, error) {
	p := &Provider{Provider: provider, APIKey: apiKey, Log: log}
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
	l := p.Log.WithName("chat").WithValues("model", modelName, "id", uuid.New().String())
	chat := &Chat{
		ctx:                p.Client.ctx,
		Send:               make(chan string),
		Recv:               make(chan string),
		GenerationComplete: make(chan bool),
		Done:               make(chan bool),
		Logger:             l,
	}
	model := NewModel(p, modelName, l)
	for _, tool := range toolsToUse {
		model.AddTool(tool)
	}
	go model.chat(chat.ctx, chat)

	return chat
}

func (p *Provider) Generate(modelName string, prompt string) (string, error) {
	l := p.Log.WithName("generate").WithValues("model", modelName, "id", uuid.New().String())
	model := NewModel(p, modelName, l)
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
		p.Log.Info("Running tool", "toolName", toolName, "args", args)
	}
	var result any
	switch p.Provider {
	case GEMINI:
		result, err = tools.RunGeminiTool(toolName, args)
	}
	if DEBUG {
		p.Log.Info("Tool result", "result", result)
	}
	return result, err
}
