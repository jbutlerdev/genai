package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jbutlerdev/genai"
	"github.com/jbutlerdev/genai/tools"
)

// readConfig reads the YAML configuration file and extracts provider and model
func readConfig() (provider, model string) {
	file, err := os.Open("/root/.config/mule/config-override-local.yml")
	if err != nil {
		return "", ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "provider:") {
			provider = strings.TrimSpace(strings.TrimPrefix(line, "provider:"))
		} else if strings.HasPrefix(line, "model:") {
			model = strings.TrimSpace(strings.TrimPrefix(line, "model:"))
		}
	}

	return provider, model
}

func main() {
	fmt.Println("Memory Tool Example")
	fmt.Println("===================")
	fmt.Println("Environment variables:")
	fmt.Println("- DATABASE_URL: PostgreSQL connection string (required)")
	fmt.Println("- EMBEDDING_PROVIDER: openai (default), gemini, or ollama (optional)")
	fmt.Println("- OPENAI_API_KEY: OpenAI API key (required if using OpenAI, not needed for LM Studio)")
	fmt.Println()

	// Check if DATABASE_URL is set
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set. Please set it to a valid PostgreSQL connection string.")
	}

	// Read configuration from config file
	configProvider, configModel := readConfig()
	
	// Get embedding provider from environment or config file or default to "openai"
	embeddingProvider := os.Getenv("EMBEDDING_PROVIDER")
	if embeddingProvider == "" {
		if configProvider != "" {
			// Treat lmstudio as openai-compatible provider
				embeddingProvider = configProvider
		} else {
			embeddingProvider = "openai"
		}
	}

	fmt.Printf("Using database URL: [REDACTED - password hidden]\n")
	// Hide password in logs for security
	displayURL := databaseURL
	if strings.Contains(displayURL, ":") && strings.Contains(displayURL, "@") {
		parts := strings.SplitN(displayURL, ":", 3)
		if len(parts) >= 3 {
			userPart := parts[1][2:] // Skip "//"
			if strings.Contains(userPart, ":") {
				userPass := strings.SplitN(userPart, "@", 2)
				if len(userPass) >= 2 {
					displayURL = strings.Replace(displayURL, ":"+userPass[0]+"@", ":***@", 1)
				}
			}
		}
	}
	fmt.Printf("Connection info: %s\n", displayURL)
	fmt.Printf("Using embedding provider: %s\n", embeddingProvider)
	if configModel != "" {
		fmt.Printf("Using embedding model from config: %s\n", configModel)
	}

	// Check if required environment variables are set
	switch embeddingProvider {
	case "openai":
		// For lmstudio (openai-compatible), we don't need an API key
		configProvider, _ := readConfig()
		if configProvider != "lmstudio" && os.Getenv("OPENAI_API_KEY") == "" {
			log.Fatal("OPENAI_API_KEY environment variable is not set. Please set it to your OpenAI API key.")
		}
	case "gemini":
		if os.Getenv("GEMINI_API_KEY") == "" {
			log.Fatal("GEMINI_API_KEY environment variable is not set. Please set it to your Gemini API key.")
		}
	case "ollama":
		if os.Getenv("OLLAMA_BASE_URL") == "" {
			fmt.Println("Warning: OLLAMA_BASE_URL environment variable is not set. Using default: http://localhost:11434")
			os.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")
		}
	}

	// Create memory tool configuration
	config := tools.MemoryConfig{
		DatabaseURL:       databaseURL,
		EmbeddingProvider: embeddingProvider,
		EmbeddingDims:     1536,                     // Default dimension
		DefaultTopK:       5,
	}

	// Read model from config file
	var configModelFromConfig string
	_, configModelFromConfig = readConfig()
	if configModelFromConfig != "" {
		config.EmbeddingModel = configModelFromConfig
	} else {
		// Fallback to default model
		config.EmbeddingModel = "lmstudio/text-embedding-qwen3-embedding-8b"
	}

	// Debug: Print configuration
	fmt.Printf("DEBUG: Configuration:\n")
	fmt.Printf("DEBUG:   DatabaseURL: %s\n", databaseURL)
	fmt.Printf("DEBUG:   EmbeddingProvider: %s\n", embeddingProvider)
	fmt.Printf("DEBUG:   EmbeddingModel: %s\n", config.EmbeddingModel)
	fmt.Printf("DEBUG:   EmbeddingDims: %d\n", config.EmbeddingDims)
	

	// Create an embedding provider
	var provider *genai.Provider
	var err error

	switch embeddingProvider {
	case "openai":
		// Check if we're using lmstudio (openai-compatible)
		configProvider, configModel := readConfig()
		fmt.Printf("DEBUG: OpenAI provider, configProvider: %s, configModel: %s\n", configProvider, configModel)
		if configProvider == "lmstudio" {
			// Use LM Studio as OpenAI-compatible provider
			baseURL := os.Getenv("LMSTUDIO_BASE_URL")
			if baseURL == "" {
				baseURL = "http://localhost:1234/v1"
			}
			fmt.Printf("DEBUG: Using LM Studio with baseURL: %s\n", baseURL)
			provider, err = genai.NewProvider(genai.OPENAI, genai.ProviderOptions{
				BaseURL:        baseURL,
				APIKey:         "lm-studio", // Dummy key for LM Studio
				EmbeddingModel: config.EmbeddingModel,
			})
		} else {
			baseURL := os.Getenv("OPENAI_BASE_URL")
			// Standard OpenAI provider
			fmt.Printf("DEBUG: Using OpenAI with baseURL: %s\n", baseURL)
			provider, err = genai.NewProvider(genai.OPENAI, genai.ProviderOptions{
				BaseURL:        baseURL,
				APIKey:         os.Getenv("OPENAI_API_KEY"),
				EmbeddingModel: config.EmbeddingModel,
			})
		}
	case "gemini":
		_, configModel := readConfig()
		fmt.Printf("DEBUG: Gemini provider, configModel: %s\n", configModel)
		provider, err = genai.NewProvider(genai.GEMINI, genai.ProviderOptions{
			APIKey:         os.Getenv("GEMINI_API_KEY"),
			EmbeddingModel: config.EmbeddingModel,
		})
	case "ollama":
		_, configModel := readConfig()
		baseURL := os.Getenv("OLLAMA_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		fmt.Printf("DEBUG: Ollama provider, configModel: %s, baseURL: %s\n", configModel, baseURL)
		provider, err = genai.NewProvider(genai.OLLAMA, genai.ProviderOptions{
			BaseURL:        baseURL,
			EmbeddingModel: config.EmbeddingModel,
		})
	default:
		log.Fatalf("Unsupported embedding provider: %s", embeddingProvider)
	}

	if err != nil {
		log.Fatalf("Failed to create embedding provider: %v", err)
	}
	
	// Debug: Print provider info
	fmt.Printf("DEBUG: Created provider: %+v\n", provider)

	// Create an embedding provider that implements the tools.EmbeddingProvider interface
	embeddingProviderImpl := &EmbeddingProviderAdapter{provider: provider}

	// Initialize the memory tool
	err = tools.InitializeMemoryTool(config, embeddingProviderImpl)
	if err != nil {
		log.Fatalf("Failed to initialize memory tool: %v", err)
	}
	fmt.Printf("config %+v\n", config)
	fmt.Println("Memory tool initialized successfully!")

	// Example 1: Store a memory
	fmt.Println("\nStoring memory...")
	storeArgs := map[string]any{
		"content": "User preference: prefers dark mode UI",
		"metadata": map[string]any{
			"type":    "user_preference",
			"user_id": "12345",
		},
	}

	storeResult, err := tools.GetTool("memory_store")
	if err != nil {
		log.Fatalf("Failed to get memory_store tool: %v", err)
	}

	result, err := storeResult.Run(storeArgs)
	if err != nil {
		log.Fatalf("Failed to store memory: %v", err)
	}

	memoryID, ok := result["id"].(string)
	if !ok {
		log.Fatal("Failed to get memory ID from result")
	}
	fmt.Printf("Stored memory with ID: %s\n", memoryID)

	// Example 2: Retrieve memories
	fmt.Println("\nRetrieving memories...")
	retrieveArgs := map[string]any{
		"query": "What are the user's UI preferences?",
		"top_k": 5,
		"filters": map[string]any{
			"user_id": "12345",
		},
	}

	retrieveTool, err := tools.GetTool("memory_retrieve")
	if err != nil {
		log.Fatalf("Failed to get memory_retrieve tool: %v", err)
	}

	retrieveResult, err := retrieveTool.Run(retrieveArgs)
	if err != nil {
		log.Fatalf("Failed to retrieve memories: %v", err)
	}

	resultsInterface, ok := retrieveResult["results"]
	if !ok {
		log.Fatal("Failed to get results from retrieve result")
	}

	resultsSlice, ok := resultsInterface.([]map[string]any)
	if !ok {
		log.Fatal("Failed to assert results to []map[string]any")
	}

	fmt.Printf("Found %d memories:\n", len(resultsSlice))
	for _, result := range resultsSlice {
		content, _ := result["content"].(string)
		similarity, _ := result["similarity"].(float64)
		fmt.Printf("- %s (similarity: %.2f)\n", content, similarity)
	}

	// Example 3: Update a memory
	fmt.Println("\nUpdating memory...")
	updateArgs := map[string]any{
		"id":      memoryID,
		"content": "User preference: prefers light mode UI",
		"metadata": map[string]any{
			"type":    "user_preference",
			"user_id": "12345",
			"updated": true,
		},
	}

	updateTool, err := tools.GetTool("memory_update")
	if err != nil {
		log.Fatalf("Failed to get memory_update tool: %v", err)
	}

	_, err = updateTool.Run(updateArgs)
	if err != nil {
		log.Fatalf("Failed to update memory: %v", err)
	}

	fmt.Println("Memory updated successfully")

	// Example 4: Delete a memory
	fmt.Println("\nDeleting memory...")
	deleteArgs := map[string]any{
		"id": memoryID,
	}

	deleteTool, err := tools.GetTool("memory_delete")
	if err != nil {
		log.Fatalf("Failed to get memory_delete tool: %v", err)
	}

	_, err = deleteTool.Run(deleteArgs)
	if err != nil {
		log.Fatalf("Failed to delete memory: %v", err)
	}

	fmt.Println("Memory deleted successfully")

	// Example 5: Using the single operation tool
	fmt.Println("\nUsing single operation tool...")
	operationArgs := map[string]any{
		"operation": "store",
		"arguments": map[string]any{
			"content": "Another test memory",
			"metadata": map[string]any{
				"type": "test",
			},
		},
	}

	operationTool, err := tools.GetTool("memory_operation")
	if err != nil {
		log.Fatalf("Failed to get memory_operation tool: %v", err)
	}

	opResult, err := operationTool.Run(operationArgs)
	if err != nil {
		log.Fatalf("Failed to run memory operation: %v", err)
	}

	fmt.Printf("Operation result: %+v\n", opResult)
}

// EmbeddingProviderAdapter adapts a genai.Provider to implement tools.EmbeddingProvider
type EmbeddingProviderAdapter struct {
	provider *genai.Provider
}

// GenerateEmbedding generates an embedding for a single text input
func (e *EmbeddingProviderAdapter) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	fmt.Printf("DEBUG: Generating embedding for text: %s\n", text[:min(50, len(text))])
	fmt.Printf("DEBUG: Provider info: %+v\n", e.provider)
	
	embedding, err := e.provider.GenerateEmbedding(ctx, text)
	if err != nil {
		fmt.Printf("DEBUG: Error generating embedding: %v\n", err)
		return nil, err
	}
	fmt.Printf("DEBUG: Successfully generated embedding\n")
	return embedding, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GenerateEmbeddings generates embeddings for multiple text inputs
func (e *EmbeddingProviderAdapter) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := e.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}
