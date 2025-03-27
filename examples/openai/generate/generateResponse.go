package main

import (
	"fmt"
	"log"
	"os"

	"github.com/go-logr/stdr"
	"github.com/jbutlerdev/genai"
)

func main() {
	prompt := "What is AI?.\n"

	openaiBaseUrl := os.Getenv("OPENAI_BASE_URL")
	if openaiBaseUrl == "" {
		panic("OPENAI_BASE_URL is not set")
	}

	openaiApiKey := os.Getenv("OPENAI_API_KEY")
	if openaiApiKey == "" {
		panic("OPENAI_API_KEY is not set")
	}

	model := "deepseek/deepseek-chat-v3-0324:free"

	log := stdr.New(log.New(os.Stdout, "", log.LstdFlags))

	openaiProvider, err := genai.NewProviderWithLog(genai.OPENAI, genai.ProviderOptions{
		BaseURL: openaiBaseUrl,
		APIKey:  openaiApiKey,
		Log:     log,
	})
	if err != nil {
		panic(err)
	}

	response, err := openaiProvider.Generate(genai.ModelOptions{
		ModelName: model,
	}, prompt)
	if err != nil {
		panic(err)
	}

	fmt.Println("Response:")
	fmt.Println(response)
}
