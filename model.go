package genai

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/jbutlerdev/genai/tools"
	ollama "github.com/ollama/ollama/api"

	gemini "github.com/google/generative-ai-go/genai"
	"github.com/openai/openai-go"
)

type ModelOptions struct {
	ModelName    string
	SystemPrompt string
}

type Model struct {
	Provider      *Provider
	Gemini        *gemini.GenerativeModel
	geminiSession *gemini.ChatSession
	ollamaClient  *ollama.Client
	ollamaModel   string
	openAIModel   string
	openAIClient  *OpenAIClient
	Tools         []*tools.Tool
	Logger        logr.Logger
	SystemPrompt  string
}

func NewModel(provider *Provider, modelOptions ModelOptions, log logr.Logger) *Model {
	m := &Model{
		Provider:     provider,
		Logger:       log,
		SystemPrompt: modelOptions.SystemPrompt,
	}
	switch provider.Provider {
	case GEMINI:
		m.Gemini = provider.Client.Gemini.GenerativeModel(modelOptions.ModelName)
		if modelOptions.SystemPrompt != "" {
			m.Gemini.SystemInstruction = gemini.NewUserContent(gemini.Text(modelOptions.SystemPrompt))
		}
	case OLLAMA:
		m.ollamaModel = modelOptions.ModelName
	case OPENAI:
		m.openAIModel = modelOptions.ModelName
		m.openAIClient = provider.Client.OpenAI
	}
	return m
}

func (m *Model) AddTool(toolsToAdd ...*tools.Tool) error {
	for _, tool := range toolsToAdd {
		switch m.Provider.Provider {
		case GEMINI:
			geminiTool, err := tools.GetGeminiTool(tool.Name)
			if err != nil {
				return err
			}
			m.Gemini.Tools = append(m.Gemini.Tools, geminiTool)
		case OLLAMA:
			m.Tools = append(m.Tools, tool)
		case OPENAI:
			m.Tools = append(m.Tools, tool)
		}
	}
	return nil
}

func (m *Model) generate(prompt string) (string, error) {
	switch m.Provider.Provider {
	case GEMINI:
		input := &retryableGeminiCallInput{
			ctx:   context.Background(),
			model: m,
			part:  gemini.Text(prompt),
		}
		m.Logger.Info("Generating content", "content", prompt)
		resp, err := retryableGeminiCall(input, 0, 1*time.Second)
		if err != nil {
			return "", fmt.Errorf("failed to generate content: %v", err)
		}
		response := handleGeminiText(resp)
		m.Logger.Info("Generated content", "content", response)
		return response, nil
	case OLLAMA:
		m.Logger.Info("Generating content with Ollama", "content", prompt)
		resp, err := ollamaGenerate(m, prompt)
		if err != nil {
			return "", fmt.Errorf("failed to generate content with Ollama: %v", err)
		}
		m.Logger.Info("Generated content", "content", resp)
		return resp, nil
	case OPENAI:
		m.Logger.Info("Generating content with OpenAI", "content", prompt)
		resp, err := m.openAIClient.Generate(context.Background(), m.openAIModel, prompt)
		if err != nil {
			return "", fmt.Errorf("failed to generate content with OpenAI: %v", err)
		}
		m.Logger.Info("Generated content", "content", resp)
		return resp, nil
	default:
		return "", fmt.Errorf("unsupported provider: %s", m.Provider.Provider)
	}
}

func (m *Model) chat(ctx context.Context, chat *Chat) error {
	m.Logger.Info("Starting chat")
	switch m.Provider.Provider {
	case GEMINI:
		m.geminiSession = m.Gemini.StartChat()
		for {
			select {
			case msg := <-chat.Send:
				m.Logger.Info("Sending message", "content", msg)
				input := &retryableGeminiCallInput{
					ctx:     ctx,
					model:   m,
					session: m.geminiSession,
					part:    gemini.Text(msg),
				}
				res, err := retryableGeminiCall(input, 0, 1*time.Second)
				if err != nil {
					m.Logger.Error(err, "Failed to send message")
					break
				}
				err = handleGeminiResponse(m, chat, res)
				if err != nil {
					m.Logger.Error(err, "Failed to handle response")
				}
			case <-chat.Done:
				return nil
			}
			chat.GenerationComplete <- true
		}
	case OLLAMA:
		return ollamaChat(m, chat)
	case OPENAI:
		// Initialize messages array
		messages := []openai.ChatCompletionMessage{}

		// Pass tools to OpenAI client
		m.openAIClient.Tools = m.Tools

		// Delegate to OpenAI client's Chat method
		return m.openAIClient.Chat(ctx, m.openAIModel, m.SystemPrompt, chat, m.Provider, messages)
	default:
		return fmt.Errorf("unsupported provider: %s", m.Provider.Provider)
	}
}
