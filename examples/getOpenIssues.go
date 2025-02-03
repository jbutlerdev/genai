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

	apiToken := os.Getenv("GEMINI_API_KEY")
	if apiToken == "" {
		panic("GEMINI_API_KEY is not set")
	}

	model := "gemini-2.0-flash-exp"

	geminiProvider, err := genai.NewProvider(genai.GEMINI, apiToken)
	if err != nil {
		panic(err)
	}

	tools, err := tools.GetTools([]string{"getAssignedPRs", "getAssignedIssues", "getContributedRepos", "getUserRepos"})
	if err != nil {
		panic(err)
	}

	chat := geminiProvider.Chat(model, tools)

	go func() {
		for msg := range chat.Recv {
			fmt.Println(msg)
			chat.Done <- true
		}
	}()

	chat.Send <- prompt
	<-chat.Done
}
