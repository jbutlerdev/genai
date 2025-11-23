package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jbutlerdev/genai"
	"github.com/jbutlerdev/genai/tools"
)

func main() {
	fmt.Println("Testing genai library...")

	// Create a provider
	provider, err := genai.NewProviderWithLog(genai.OPENAI, genai.ProviderOptions{
		Name:          "test",
		APIKey:        "test-key",
		BaseURL:       "https://bifrost.butler.ooo/v1",
		EmbeddingModel: "lmstudio/text-embedding-qwen3-embedding-8b",
	})
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	fmt.Printf("Provider created: %+v\n", provider)

	// Test generating an embedding
	ctx := context.Background()
	embedding, err := provider.GenerateEmbedding(ctx, "test text")
	if err != nil {
		log.Printf("Failed to generate embedding: %v", err)
	} else {
		fmt.Printf("Generated embedding with %d dimensions\n", len(embedding))
	}

	// Test the memory tool
	config := tools.MemoryConfig{
		DatabaseURL:       "postgres://test:test@localhost:5432/test",
		EmbeddingProvider: "openai",
		EmbeddingModel:    "lmstudio/text-embedding-qwen3-embedding-8b",
		EmbeddingDims:     1536,
		DefaultTopK:       5,
	}

	// Create an embedding provider that implements the tools.EmbeddingProvider interface
	embeddingProviderImpl := &EmbeddingProviderAdapter{provider: provider}

	// Initialize the memory tool
	err = tools.InitializeMemoryTool(config, embeddingProviderImpl)
	if err != nil {
		log.Printf("Failed to initialize memory tool: %v", err)
	} else {
		fmt.Println("Memory tool initialized successfully")
	}
}

// EmbeddingProviderAdapter adapts a genai.Provider to implement tools.EmbeddingProvider
type EmbeddingProviderAdapter struct {
	provider *genai.Provider
}

// GenerateEmbedding generates an embedding for a single text input
func (e *EmbeddingProviderAdapter) GenerateEmbedding(ctx context.Context, text string, model string) ([]float32, error) {
	return e.provider.GenerateEmbedding(ctx, text, model)
}

// GenerateEmbeddings generates embeddings for multiple text inputs
func (e *EmbeddingProviderAdapter) GenerateEmbeddings(ctx context.Context, texts []string, model string) ([][]float32, error) {
	return e.provider.GenerateEmbeddings(ctx, texts, model)
}