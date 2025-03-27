package main

import (
	"log"
	"os"

	"github.com/go-logr/stdr"
	"github.com/jbutlerdev/genai"
	"github.com/jbutlerdev/genai/tools"
)

func main() {
	prompt := "Provide me a list of all open issues that I have been assigned to.\n" +
		"My github username is jbutlerdev.\n"
	openaiBaseUrl := os.Getenv("OPENAI_BASE_URL")
	if openaiBaseUrl == "" {
		panic("OPENAI_BASE_URL is not set")
	}

	openaiApiKey := os.Getenv("OPENAI_API_KEY")
	if openaiApiKey == "" {
		panic("OPENAI_API_KEY is not set")
	}

	// model := "deepseek/deepseek-chat-v3-0324:free"
	model := "mistralai/mistral-small-3.1-24b-instruct:free"

	log := stdr.New(log.New(os.Stdout, "", log.LstdFlags))

	openaiProvider, err := genai.NewProviderWithLog(genai.OPENAI, genai.ProviderOptions{
		BaseURL: openaiBaseUrl,
		APIKey:  openaiApiKey,
		Log:     log,
	})
	if err != nil {
		panic(err)
	}
	tools, err := tools.GetTools([]string{"getAssignedPRs", "getAssignedIssues", "getContributedRepos", "getUserRepos"})
	if err != nil {
		panic(err)
	}
	chat := openaiProvider.Chat(genai.ModelOptions{
		ModelName: model,
	}, tools)

	go func() {
		log.Info("Starting to receive messages")
		for msg := range chat.Recv {
			log.Info("Received message", "message", msg)
			<-chat.GenerationComplete
		}
		log.Info("Done")
	}()

	chat.Send <- prompt

	<-chat.GenerationComplete
	chat.Done <- true
}
