package main

import (
	"fmt"
	"os"

	"github.com/jbutlerdev/genai"
)

func main() {
	prompt := "What is AI?.\n"

	ollamaBaseUrl := os.Getenv("OLLAMA_BASE_URL")
	if ollamaBaseUrl == "" {
		panic("OLLAMA_BASE_URL is not set")
	}

	model := "qwen2.5:7b-instruct-q6_K"

	ollamaProvider, err := genai.NewProvider(genai.OLLAMA, genai.ProviderOptions{
		BaseURL: ollamaBaseUrl,
	})
	if err != nil {
		panic(err)
	}

	response, err := ollamaProvider.Generate(model, prompt)
	if err != nil {
		panic(err)
	}

	fmt.Println("Response:")
	fmt.Println(response)
}
