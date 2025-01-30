package genai

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	gemini "github.com/google/generative-ai-go/genai"
)

const (
	RETRY_COUNT = 9
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
		if strings.Contains(err.Error(), "429") {
			log.Printf("Rate limit exceeded, waiting for %v and retrying. Current attempt: %d", delay, attempt)
			// rate limit exceeded, wait and retry
			time.Sleep(delay)
			// create exponential backoff
			nextDelay := delay * time.Duration(math.Pow(2, float64(attempt)))
			return retryableGeminiCall(input, attempt+1, nextDelay)
		}
		// non-retryable error
		return nil, fmt.Errorf("failed to get response: %v", err)
	}
	return resp, nil
}

func handleGeminiResponse(m *Model, chat *Chat, resp *gemini.GenerateContentResponse) error {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				switch p := part.(type) {
				case gemini.FunctionCall:
					if DEBUG {
						log.Println("Handling function call", p)
					}
					resp, err := handleGeminiFunctionCall(m, &p)
					if err != nil {
						return fmt.Errorf("failed to handle function call: %v", err)
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
					mresp, err := retryableGeminiCall(input, 0, 1*time.Second)
					if err != nil {
						return fmt.Errorf("failed to send message: %v", err)
					}
					return handleGeminiResponse(m, chat, mresp)
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

func handleGeminiFunctionCall(m *Model, f *gemini.FunctionCall) (gemini.Part, error) {
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
