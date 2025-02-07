package main

import (
	"fmt"
	"os"

	"github.com/jbutlerdev/genai"
)

func main() {
	ollamaBaseUrl := os.Getenv("OLLAMA_BASE_URL")
	if ollamaBaseUrl == "" {
		panic("OLLAMA_BASE_URL is not set")
	}

	ollamaProvider, err := genai.NewProvider(genai.OLLAMA, genai.ProviderOptions{
		BaseURL: ollamaBaseUrl,
	})
	if err != nil {
		panic(err)
	}

	models := ollamaProvider.Models()
	for _, model := range models {
		fmt.Println(model)
	}
}
