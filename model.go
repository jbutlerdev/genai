package genai

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/jbutlerdev/genai/tools"
	ollama "github.com/ollama/ollama/api"

	gemini "github.com/google/generative-ai-go/genai"
)

type Model struct {
	Provider      *Provider
	Gemini        *gemini.GenerativeModel
	geminiSession *gemini.ChatSession
	ollamaClient  *ollama.Client
	ollamaModel   string
	Tools         []*tools.Tool
	Logger        logr.Logger
}

func NewModel(provider *Provider, model string, log logr.Logger) *Model {
	m := &Model{Provider: provider, Logger: log}
	switch provider.Provider {
	case GEMINI:
		m.Gemini = provider.Client.Gemini.GenerativeModel(model)
	case OLLAMA:
		m.ollamaModel = model
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
		resp, err := ollamaGenerate(m.Provider.Client.Ollama, m.ollamaModel, prompt)
		if err != nil {
			return "", fmt.Errorf("failed to generate content with Ollama: %v", err)
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
	default:
		return fmt.Errorf("unsupported provider: %s", m.Provider.Provider)
	}
}
