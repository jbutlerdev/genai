package genai

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	gemini "github.com/google/generative-ai-go/genai"
)

const (
	RETRY_COUNT     = 8
	MAX_RETRY_DELAY = 30 * time.Second
)

type retryableGeminiCallInput struct {
	ctx     context.Context
	model   *Model
	part    gemini.Part
	session *gemini.ChatSession
}

func retryableGeminiCall(input *retryableGeminiCallInput, attempt int, delay time.Duration) (*gemini.GenerateContentResponse, error) {
	if attempt > RETRY_COUNT {
		return nil, fmt.Errorf("failed to get response after %d attempts", RETRY_COUNT)
	}
	var resp *gemini.GenerateContentResponse
	var err error
	if input.session == nil {
		resp, err = input.model.Gemini.GenerateContent(input.ctx, input.part)
	} else {
		resp, err = input.session.SendMessage(input.ctx, input.part)
	}
	if err != nil {
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "503") || strings.Contains(err.Error(), "400") {
			input.model.Logger.Error(err, "Retryable error", "delay", delay, "attempt", attempt)
			// rate limit exceeded, wait and retry
			time.Sleep(delay)
			delay = min(delay*2, MAX_RETRY_DELAY)
			return retryableGeminiCall(input, attempt+1, delay)
		}
		// non-retryable error
		return nil, fmt.Errorf("failed to get response: %v", err)
	}
	return resp, nil
}

func handleGeminiResponse(m *Model, chat *Chat, resp *gemini.GenerateContentResponse) error {
	m.Logger.Info("total_token_count", "content", strconv.Itoa(int(resp.UsageMetadata.TotalTokenCount)))
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				switch p := part.(type) {
				case gemini.FunctionCall:
					m.Logger.Info("Handling function call", "name", p.Name, "content", fmt.Sprintf("%v", part))
					resp, err := handleGeminiFunctionCall(m, &p)
					if err != nil {
						m.Logger.Error(err, "failed to handle function call")
					}
					if resp == nil {
						return nil
					}
					input := &retryableGeminiCallInput{
						ctx:     chat.ctx,
						model:   m,
						session: m.geminiSession,
						part:    resp,
					}
					m.Logger.Info("Sending function call output", "name", p.Name, "content", fmt.Sprintf("%v", input.part))
					mresp, err := retryableGeminiCall(input, 0, 1*time.Second)
					if err != nil {
						return fmt.Errorf("failed to send message: %v", err)
					}
					return handleGeminiResponse(m, chat, mresp)
				case gemini.Text:
					m.Logger.Info("Handling text", "content", fmt.Sprintf("%v", part))
					chat.Recv <- fmt.Sprintf("%v", part)
				default:
					return fmt.Errorf("unexpected part: %v", part)
				}
			}
		}
	}
	return nil
}

func handleGeminiFunctionCall(m *Model, f *gemini.FunctionCall) (gemini.Part, error) {
	resp, err := m.Provider.RunTool(f.Name, f.Args)
	if err != nil {
		m.Logger.Error(err, "failed to run tool")
	}
	part, ok := resp.(gemini.FunctionResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %v", resp)
	}
	return part, nil
}

func handleGeminiText(resp *gemini.GenerateContentResponse) string {
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

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
