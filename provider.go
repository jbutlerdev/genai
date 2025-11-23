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
	Provider      string `json:"provider"`
	Name          string `json:"name"`
	APIKey        string `json:"apiKey"`
	BaseURL       string `json:"baseURL"`
	Client        *Client
	Model         *Model
	EmbeddingModel string
	Log           logr.Logger
}

type ProviderOptions struct {
	Name          string
	APIKey        string
	BaseURL       string
	EmbeddingModel string
	Log           logr.Logger
}

type Chat struct {
	ctx                context.Context
	Send               chan string
	Recv               chan string
	GenerationComplete chan bool
	Done               chan bool
	Logger             logr.Logger
	Turns              int
}

// NewProvider creates a new provider with a default logr.Discard() logger
func NewProvider(provider string, options ProviderOptions) (*Provider, error) {
	p := &Provider{
		Provider:       provider,
		Name:           options.Name,
		APIKey:         options.APIKey,
		BaseURL:        options.BaseURL,
		EmbeddingModel: options.EmbeddingModel,
		Log:            logr.Discard(),
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
		Provider:       provider,
		Name:           options.Name,
		APIKey:         options.APIKey,
		BaseURL:        options.BaseURL,
		EmbeddingModel: options.EmbeddingModel,
		Log:            options.Log,
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

func (p *Provider) Chat(modelOptions ModelOptions, toolsToUse []*tools.Tool) *Chat {
	l := p.Log.WithName("chat").WithValues("model", modelOptions.ModelName, "id", uuid.New().String())
	chat := &Chat{
		ctx:                p.Client.ctx,
		Send:               make(chan string),
		Recv:               make(chan string),
		GenerationComplete: make(chan bool),
		Done:               make(chan bool),
		Logger:             l,
	}
	model := NewModel(p, modelOptions, l)
	for _, tool := range toolsToUse {
		model.AddTool(tool)
	}
	go model.chat(chat.ctx, chat)

	return chat
}

func (p *Provider) Generate(modelOptions ModelOptions, prompt string) (string, error) {
	l := p.Log.WithName("generate").WithValues("model", modelOptions.ModelName, "id", uuid.New().String())
	model := NewModel(p, modelOptions, l)
	switch p.Provider {
	case OLLAMA:
		model.ollamaClient = p.Client.Ollama
	case OPENAI:
		model.openAIClient = p.Client.OpenAI
	}
	return model.generate(prompt, modelOptions)
}

func (p *Provider) RunTool(toolName string, args map[string]any) (any, error) {
	tool, err := tools.GetTool(toolName)
	if err != nil {
		return err.Error(), err
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
	case OPENAI:
		if tool.Run != nil {
			result, err = tool.Run(args)
		} else {
			err = fmt.Errorf("tool %s does not have a run function", toolName)
		}
	}
	if DEBUG {
		p.Log.Info("Tool result", "result", result)
	}
	if tool.Summarize {
		return p.Generate(ModelOptions{
			ModelName: "llamacpp/qwen3-30b-a3b",
			Parameters: map[string]any{
				NumPredict: 5000,
			},
		}, fmt.Sprintf(`Summarize these tool results in 5000 words or less. Your summarization must be shorter than the provided value\n
				If there appears to be an error, just return the error with no additional information\n
				Do not provide any reference to the word count or the fact that you summarized. Simply return your content.\n\n%s`, result))
	}
	return result, err
}

// GenerateEmbedding generates an embedding for a single text input using the appropriate provider
func (p *Provider) GenerateEmbedding(ctx context.Context, text string, model string) ([]float32, error) {
	switch p.Provider {
	case GEMINI:
		return geminiGenerateEmbedding(ctx, p.Client.Gemini, text, model)
	case OPENAI:
		return p.Client.OpenAI.GenerateEmbedding(ctx, text, model)
	case OLLAMA:
		return ollamaGenerateEmbedding(ctx, p.Client.Ollama, text, model)
	default:
		return nil, fmt.Errorf("unsupported provider for embeddings: %s", p.Provider)
	}
}

// GenerateEmbeddings generates embeddings for multiple text inputs using the appropriate provider
func (p *Provider) GenerateEmbeddings(ctx context.Context, texts []string, model string) ([][]float32, error) {
	switch p.Provider {
	case GEMINI:
		return geminiGenerateEmbeddings(ctx, p.Client.Gemini, texts, model)
	case OPENAI:
		return p.Client.OpenAI.GenerateEmbeddings(ctx, texts, model)
	case OLLAMA:
		return ollamaGenerateEmbeddings(ctx, p.Client.Ollama, texts, model)
	default:
		return nil, fmt.Errorf("unsupported provider for embeddings: %s", p.Provider)
	}
}
