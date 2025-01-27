package main

import (
	"context"
	"fmt"
	"os"
	"tools"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type chatInput struct {
	Prompt string
	Model  string
	APIKey string
	Tools  []*genai.Tool
}

func getTools(toolNames []string) []*genai.Tool {
	geminiProvider := tools.NewGenAIProvider(tools.GEMINI)
	geminiTools := make([]*genai.Tool, len(toolNames))
	for i, toolName := range toolNames {
		geminiTool, err := geminiProvider.GetTool(toolName)
		if err != nil || geminiTool == nil {
			panic(err)
		}
		geminiTools[i] = geminiTool.GeminiTool
	}
	return geminiTools
}

func geminiChat(input chatInput) error {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(input.APIKey))
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %v", err)
	}
	defer client.Close()

	model := client.GenerativeModel(input.Model)
	model.Tools = input.Tools
	session := model.StartChat()

	res, err := session.SendMessage(ctx, genai.Text(input.Prompt))
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	for {
		part, err := handleResponse(res)
		if err != nil {
			return fmt.Errorf("failed to handle response: %v", err)
		}

		if part == nil {
			// no user output, ending the chat
			break
		}

		// Provide the model with user output.
		if tools.DEBUG {
			fmt.Println("Sending response", part)
		}
		res, err = session.SendMessage(ctx, part)
		if err != nil {
			return fmt.Errorf("failed to send response: %v", err)
		}
	}
	return nil
}

func handleResponse(resp *genai.GenerateContentResponse) (genai.Part, error) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			part := cand.Content.Parts[0]
			switch p := part.(type) {
			case genai.FunctionCall:
				return handleFunctionCall(&p)
			case genai.Text:
				handleText(resp)
				return nil, nil
			default:
				return nil, fmt.Errorf("unexpected part: %v", part)
			}
		}
	}
	return nil, nil
}

func handleFunctionCall(f *genai.FunctionCall) (genai.Part, error) {
	resp, err := tools.NewGenAIProvider(tools.GEMINI).RunTool(f.Name, f.Args)
	if err != nil {
		return nil, fmt.Errorf("failed to run tool: %v", err)
	}
	part, ok := resp.(genai.FunctionResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %v", resp)
	}
	return part, nil
}

func handleText(resp *genai.GenerateContentResponse) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				fmt.Println(part)
			}
		}
	}
	fmt.Println("---")
}

func main() {
	/*
		prompt := "Provide a list of all of the current git directories that I have cloned.\n" +
			"I only want to know of the ones for my personal projects.\n" +
			"My github username is jbutlerdev and personal projects are in the jbutlerdev directory.\n" +
			"I want to know the full path to the directory.\n" +
			"There are more than 1 projects cloned, so I want to know the full path to each of them.\n" +
			"Write a bash script that displays the content of each directory in tree format."
	*/
	prompt := "Provide me a list of all open issues that I have been assigned to.\n" +
		"My github username is jbutlerdev.\n"
	input := chatInput{
		Prompt: prompt,
		Model:  "gemini-2.0-flash-exp",
		APIKey: os.Getenv("GEMINI_API_KEY"),
		Tools:  getTools([]string{"getAssignedPRs", "getAssignedIssues", "getContributedRepos", "getUserRepos"}),
	}
	err := geminiChat(input)
	if err != nil {
		fmt.Println(err)
	}
}
