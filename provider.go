package genai

import (
	"context"
	"fmt"

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
	BaseURL  string `json:"baseURL"`
	Client   *Client
	Model    *Model
	Log      logr.Logger
}

type ProviderOptions struct {
	APIKey  string
	BaseURL string
	Log     logr.Logger
}

type Chat struct {
	ctx                context.Context
	Send               chan string
	Recv               chan string
	GenerationComplete chan bool
	Done               chan bool
	Logger             logr.Logger
}

// NewProvider creates a new provider with a default logr.Discard() logger
func NewProvider(provider string, options ProviderOptions) (*Provider, error) {
	p := &Provider{
		Provider: provider,
		APIKey:   options.APIKey,
		BaseURL:  options.BaseURL,
		Log:      logr.Discard(),
	}
	client, err := NewClient(p)
	if err != nil {
		return nil, err
	}
	p.Client = client
	return p, nil
}

// NewProviderWithLog creates a new provider with a custom logr.Logger
func NewProviderWithLog(provider string, options ProviderOptions) (*Provider, error) {
	p := &Provider{
		Provider: provider,
		APIKey:   options.APIKey,
		BaseURL:  options.BaseURL,
		Log:      options.Log,
	}
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
	if p.Provider == OLLAMA {
		model.ollamaClient = p.Client.Ollama
	}
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
	case OLLAMA:
		if tool.Run != nil {
			result, err = tool.Run(args)
		} else {
			err = fmt.Errorf("tool %s does not have a run function", toolName)
		}
	}
	if DEBUG {
		p.Log.Info("Tool result", "result", result)
	}
	return result, err
}
