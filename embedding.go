package genai

import (
	"context"
)

// EmbeddingProvider defines the interface for generating embeddings
type EmbeddingProvider interface {
	// GenerateEmbedding generates an embedding for a single text input
	GenerateEmbedding(ctx context.Context, text string, model string) ([]float32, error)

	// GenerateEmbeddings generates embeddings for multiple text inputs
	GenerateEmbeddings(ctx context.Context, texts []string, model string) ([][]float32, error)
}