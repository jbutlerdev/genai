package genai

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jbutlerdev/genai/tools"

	gemini "github.com/google/generative-ai-go/genai"
)

type Model struct {
	Provider      *Provider
	Gemini        *gemini.GenerativeModel
	geminiSession *gemini.ChatSession
	Tools         []*tools.Tool
}

func NewModel(provider *Provider, model string) *Model {
	m := &Model{Provider: provider}
	switch provider.Provider {
	case GEMINI:
		m.Gemini = provider.Client.Gemini.GenerativeModel(model)
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
		}
	}
	return nil
}

func (m *Model) generate(prompt string) (string, error) {
	input := &retryableGeminiCallInput{
		ctx:   context.Background(),
		model: m,
		part:  gemini.Text(prompt),
	}
	resp, err := retryableGeminiCall(input, 0, 1*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %v", err)
	}
	return handleGeminiText(resp), nil
}

func (m *Model) chat(ctx context.Context, chat *Chat) error {
	m.geminiSession = m.Gemini.StartChat()
	for {
		select {
		case msg := <-chat.Send:
			if DEBUG {
				log.Println("Sending message", msg)
			}
			input := &retryableGeminiCallInput{
				ctx:     ctx,
				model:   m,
				session: m.geminiSession,
				part:    gemini.Text(msg),
			}
			res, err := retryableGeminiCall(input, 0, 1*time.Second)
			if err != nil {
				log.Println("Failed to send message", err)
				break
			}
			err = handleGeminiResponse(m, chat, res)
			if err != nil {
				log.Println("Failed to handle response", err)
			}
		case <-chat.Done:
			return nil
		}
		chat.GenerationComplete <- true
	}
}
