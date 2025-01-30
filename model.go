package genai

import (
	"context"
	"fmt"
	"log"

	"genai/tools"

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
	resp, err := m.Gemini.GenerateContent(context.Background(), gemini.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %v", err)
	}
	return handleText(resp), nil
}

func (m *Model) chat(ctx context.Context, chat *Chat) error {
	m.geminiSession = m.Gemini.StartChat()
	for {
		select {
		case msg := <-chat.Send:
			if DEBUG {
				log.Println("Sending message", msg)
			}
			res, err := m.geminiSession.SendMessage(ctx, gemini.Text(msg))
			if err != nil {
				log.Println("Failed to send message", err)
			}
			err = handleResponse(m, chat, res)
			if err != nil {
				log.Println("Failed to handle response", err)
			}
		case <-chat.Done:
			return nil
		}
	}
}

func handleResponse(m *Model, chat *Chat, resp *gemini.GenerateContentResponse) error {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				switch p := part.(type) {
				case gemini.FunctionCall:
					if DEBUG {
						log.Println("Handling function call", p)
					}
					resp, err := handleFunctionCall(m, &p)
					if err != nil {
						return fmt.Errorf("failed to handle function call: %v", err)
					}
					if resp == nil {
						return nil
					}
					mresp, err := m.geminiSession.SendMessage(chat.ctx, resp)
					if err != nil {
						return fmt.Errorf("failed to send message: %v", err)
					}
					return handleResponse(m, chat, mresp)
				case gemini.Text:
					if DEBUG {
						log.Println("Handling text", part)
					}
					chat.Recv <- fmt.Sprintf("%v", part)
				default:
					return fmt.Errorf("unexpected part: %v", part)
				}
			}
		}
	}
	return nil
}

func handleFunctionCall(m *Model, f *gemini.FunctionCall) (gemini.Part, error) {
	resp, err := m.Provider.RunTool(f.Name, f.Args)
	if err != nil {
		return nil, fmt.Errorf("failed to run tool: %v", err)
	}
	part, ok := resp.(gemini.FunctionResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %v", resp)
	}
	return part, nil
}

func handleText(resp *gemini.GenerateContentResponse) string {
	var text string
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				text += fmt.Sprintf("%v", part)
			}
		}
	}
	return text
}
