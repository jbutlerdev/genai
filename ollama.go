package genai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	"github.com/jbutlerdev/genai/tools"
	ollama "github.com/ollama/ollama/api"
)

var stream = false

var toolCallRegex = regexp.MustCompile(`\{"name":\s*"[^"]*",\s*"arguments":`)

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
			messages = append(messages, ollama.Message{Role: "user", Content: msg})

			// Convert tools to Ollama format
			var ollamaTools []ollama.Tool

			for _, tool := range model.Tools {
				ollamaTool, err := tools.GetOllamaTool(tool.Name)
				if err != nil {
					model.Logger.Error(err, "Failed to get Ollama tool", "tool", tool.Name)
					continue
				}
				ollamaTools = append(ollamaTools, *ollamaTool)
			}

			err := handleOllamaResponse(model, ollamaTools, chat, messages)
			if err != nil {
				model.Logger.Error(err, "Failed to handle ollama response")
			}

		case <-chat.Done:
			return nil
		}
		chat.GenerationComplete <- true
	}
}

func handleOllamaResponse(model *Model, tools []ollama.Tool, chat *Chat, messages []ollama.Message) error {
	lastMessage := messages[len(messages)-1]
	if lastMessage.Role == "tool" {
		model.Logger.Info("Sending function call output", "content", lastMessage.Content)
	} else {
		model.Logger.Info("Sending message to Ollama", "content", lastMessage.Content)
	}
	respFunc := func(resp ollama.ChatResponse) error {
		usageString := fmt.Sprintf("prompt_count: %d, eval_count: %d, total_count: %d",
			resp.PromptEvalCount, resp.EvalCount, (resp.PromptEvalCount + resp.EvalCount))
		model.Logger.Info("token usage", "content", usageString)
		messages = append(messages, resp.Message)
		return nil
	}

	err := model.Provider.Client.Ollama.Chat(model.Provider.Client.ctx, &ollama.ChatRequest{
		Model:    model.ollamaModel,
		Messages: messages,
		Tools:    tools,
		Stream:   &stream,
		Options: map[string]interface{}{
			"num_ctx": 32768,
		},
	}, respFunc)
	if err != nil {
		model.Logger.Error(err, "Failed to send message to Ollama")
		return err
	}
	lastMessage = messages[len(messages)-1]
	if len(lastMessage.ToolCalls) < 1 {
		lastMessage, err = unmarshalToolCall(lastMessage, model.Logger)
		if err != nil {
			// if we hit this case it means the model returned a message that we believe to be a tool call but it can not be unmarshalled.
			// there is an edge case here where it could be json, and not a tool call, but we will ignore that for now.
			model.Logger.Info("Received invalid tool call", "content", lastMessage.Content)
			model.Logger.Error(err, "Failed to unmarshal tool call, sending error back to Ollama")
			errorMsg := ollama.Message{Role: "user", Content: fmt.Sprintf("error: you provided an invalid tool call: %s", err.Error())}
			messages = append(messages, errorMsg)
			err = handleOllamaResponse(model, tools, chat, messages)
			return err
		}
	}
	// Handle tool calls if any
	if len(lastMessage.ToolCalls) > 0 {
		for _, toolCall := range lastMessage.ToolCalls {
			funcJson, err := json.Marshal(toolCall.Function)
			if err != nil {
				model.Logger.Error(err, "Failed to marshal tool call arguments", "tool", toolCall.Function.Name)
			}
			model.Logger.Info("Handling function call", "name", toolCall.Function.Name, "content", string(funcJson))
			result, err := model.Provider.RunTool(toolCall.Function.Name, toolCall.Function.Arguments)
			if err != nil {
				model.Logger.Error(err, "Failed to run tool", "tool", toolCall.Function.Name)
			}
			// Add tool result to chat
			resultMsg := fmt.Sprintf("Tool %s returned: %v", toolCall.Function.Name, result)
			toolResultMessage := ollama.Message{Role: "tool", Content: resultMsg}
			messages = append(messages, toolResultMessage)
		}
		// send response
		err = handleOllamaResponse(model, tools, chat, messages)
		if err != nil {
			model.Logger.Error(err, "Failed to handle tool result")
		}
	} else {
		// send response
		model.Logger.Info("Received response from Ollama", "content", lastMessage.Content)
		chat.Recv <- lastMessage.Content
	}
	return nil
}

func unmarshalToolCall(message ollama.Message, logger logr.Logger) (ollama.Message, error) {
	toolCallMatch := toolCallRegex.FindString(message.Content)
	if toolCallMatch == "" {
		// no tool call found, return original message
		return message, nil
	}
	mark := strings.Index(message.Content, toolCallMatch)
	if mark == -1 {
		// no tool call found, return original message
		return message, nil
	}
	toolString := message.Content[mark:]
	// for now assume there's nothing after the tool call
	toolString = strings.TrimSuffix(toolString, "```")
	toolString = strings.TrimSuffix(toolString, "</tool_call>")
	var toolCall ollama.ToolCallFunction
	err := json.Unmarshal([]byte(toolString), &toolCall)
	if err != nil {
		toolString = fixQuotes(toolString)
		err = json.Unmarshal([]byte(toolString), &toolCall)
		if err != nil {
			log.Printf("Failed to unmarshal tool call, attempted string: %s: %s", toolString, err.Error())
			return message, fmt.Errorf("failed to unmarshal tool call: %w", err)
		} else {
			logger.Info("Fixed quotes and unmarshalled tool call", "content", toolString)
			log.Printf("Fixed quotes and unmarshalled tool call: %s", toolString)
		}
	}
	message.ToolCalls = append(message.ToolCalls, ollama.ToolCall{
		Function: toolCall,
	})
	log.Printf("Added tool call to message: %v", toolString)
	return message, nil
}

func fixQuotes(in string) string {
	var sb strings.Builder
	approvedSecondRunes := []rune{':', ',', '}'}
	open := false
	for i, c := range in {
		if c == '"' {
			if !open {
				open = true
				sb.WriteRune(c)
			} else {
				if runeContains(approvedSecondRunes, rune(in[i+1])) {
					open = false
					sb.WriteRune(c)
				} else {
					if in[i-1] != '\\' {
						sb.WriteString(`\"`)
					} else {
						sb.WriteRune(c)
					}
				}
			}
		} else {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

func runeContains(arr []rune, i rune) bool {
	for _, r := range arr {
		if r == i {
			return true
		}
	}
	return false
}
