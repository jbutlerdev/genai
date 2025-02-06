package main

import (
	"fmt"
	"os"

	"github.com/jbutlerdev/genai"
	"github.com/jbutlerdev/genai/tools"
)

func main() {
	prompt := "Provide me a list of all open issues that I have been assigned to.\n" +
		"My github username is jbutlerdev.\n"

	ollamaBaseUrl := os.Getenv("OLLAMA_BASE_URL")
	if ollamaBaseUrl == "" {
		panic("OLLAMA_BASE_URL is not set")
	}

	model := "llama3.1:8b"

	ollamaProvider, err := genai.NewProvider(genai.OLLAMA, genai.ProviderOptions{
		BaseURL: ollamaBaseUrl,
	})
	if err != nil {
		panic(err)
	}

	tools, err := tools.GetTools([]string{"getAssignedPRs", "getAssignedIssues", "getContributedRepos", "getUserRepos"})
	if err != nil {
		panic(err)
	}

	chat := ollamaProvider.Chat(model, tools)

	go func() {
		for msg := range chat.Recv {
			fmt.Println(msg)
		}
	}()

	chat.Send <- prompt
	<-chat.GenerationComplete
	chat.Done <- true
}
