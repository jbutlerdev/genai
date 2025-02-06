package genai

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/jbutlerdev/genai/tools"
	ollama "github.com/ollama/ollama/api"
)

var stream = false

func NewOllamaClient(baseURL string) *ollama.Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	return ollama.NewClient(url, &http.Client{})
}

func ollamaGenerate(client *ollama.Client, model string, prompt string) (string, error) {
	stream := false
	req := ollama.GenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: &stream,
	}

	var respString string

	respFunc := func(resp ollama.GenerateResponse) error {
		respString = resp.Response
		return nil
	}

	err := client.Generate(context.Background(), &req, respFunc)
	if err != nil {
		return "", err
	}
	return respString, nil
}

func ollamaChat(model *Model, chat *Chat) error {
	messages := []ollama.Message{}
	for {
		select {
		case msg := <-chat.Send:
			log.Printf("Sending message to Ollama: %s", msg)
			model.Logger.Info("Sending message to Ollama", "content", msg)
			messages = append(messages, ollama.Message{Role: "user", Content: msg})

			// Convert tools to Ollama format
			var ollamaTools []ollama.Tool

			for _, tool := range model.Tools {
				ollamaTool, err := tools.GetOllamaTool(tool.Name)
				if err != nil {
					log.Printf("Failed getting ollama tool: %v", err)
					model.Logger.Error(err, "Failed to get Ollama tool", "tool", tool.Name)
					continue
				}
				ollamaTools = append(ollamaTools, *ollamaTool)
			}

			handleOllamaResponse(model, ollamaTools, chat, messages)

		case <-chat.Done:
			return nil
		}
		chat.GenerationComplete <- true
	}
}

func handleOllamaResponse(model *Model, tools []ollama.Tool, chat *Chat, messages []ollama.Message) error {
	respFunc := func(resp ollama.ChatResponse) error {
		messages = append(messages, resp.Message)
		return nil
	}

	err := model.Provider.Client.Ollama.Chat(model.Provider.Client.ctx, &ollama.ChatRequest{
		Model:    model.ollamaModel,
		Messages: messages,
		Tools:    tools,
		Stream:   &stream,
	}, respFunc)
	if err != nil {
		log.Printf("Failed to send message to Ollama: %v", err)
		model.Logger.Error(err, "Failed to send message to Ollama")
		return err
	}
	lastMessage := messages[len(messages)-1]
	// Handle tool calls if any
	if len(lastMessage.ToolCalls) > 0 {
		for _, toolCall := range lastMessage.ToolCalls {
			model.Logger.Info("Handling function call", "name", toolCall.Function.Name, "content", fmt.Sprintf("%v", toolCall.Function.Arguments))
			result, err := model.Provider.RunTool(toolCall.Function.Name, toolCall.Function.Arguments)
			if err != nil {
				log.Printf("Failed to run tool: %v", err)
				model.Logger.Error(err, "Failed to run tool", "tool", toolCall.Function.Name)
				continue
			}
			// Add tool result to chat
			resultMsg := fmt.Sprintf("Tool %s returned: %v", toolCall.Function.Name, result)
			log.Printf("Tool result: %s", resultMsg)
			model.Logger.Info("Sending function call output", "name", toolCall.Function.Name, "content", fmt.Sprintf("%v", toolCall.Function.Arguments))
			toolResultMessage := ollama.Message{Role: "tool", Content: resultMsg}
			messages = append(messages, toolResultMessage)
			// send response
			return handleOllamaResponse(model, tools, chat, messages)
		}
	}
	// send response
	model.Logger.Info("Handling text", "content", fmt.Sprintf("%v", lastMessage.Content))
	chat.Recv <- lastMessage.Content
	return nil
}
