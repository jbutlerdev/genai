package main

import (
	"fmt"
	"log"
	"os"

	"github.com/go-logr/stdr"
	"github.com/jbutlerdev/genai"
)

func main() {
	openaiBaseUrl := os.Getenv("OPENAI_BASE_URL")
	if openaiBaseUrl == "" {
		panic("OPENAI_BASE_URL is not set")
	}

	openaiApiKey := os.Getenv("OPENAI_API_KEY")
	if openaiApiKey == "" {
		panic("OPENAI_API_KEY is not set")
	}

	log := stdr.New(log.New(os.Stdout, "", log.LstdFlags))

	openaiProvider, err := genai.NewProviderWithLog(genai.OPENAI, genai.ProviderOptions{
		BaseURL: openaiBaseUrl,
		APIKey:  openaiApiKey,
		Log:     log,
	})
	if err != nil {
		panic(err)
	}

	models := openaiProvider.Models()
	for _, model := range models {
		fmt.Println(model)
	}
}
