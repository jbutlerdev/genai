# GenAI Project Context

## Project Overview

This is a Go library that provides a unified wrapper interface for various Generative AI providers. The library allows developers to easily switch between different AI providers (like Gemini, OpenAI, Ollama) while maintaining consistent interfaces for model interactions and tool usage.

### Key Features

- **Multi-provider Support**: Unified interface for Gemini, OpenAI, Ollama, and planned support for Anthropic and Azure
- **Tool Integration**: Built-in tools for file operations, GitHub interactions, and extensible tool categories
- **Chat and Generation APIs**: Both streaming chat interfaces and simple text generation
- **Automatic Retries**: Built-in retry mechanisms for rate-limited or failed API calls
- **Structured Tool Definitions**: Strongly typed tool definitions with parameter specifications

### Main Components

1. **Providers**: Abstraction layer for different AI service providers
2. **Models**: Interface for interacting with specific AI models
3. **Clients**: Low-level API clients for each provider
4. **Tools**: Pre-built functions that AI models can invoke
5. **Chats**: Streaming conversation interface

## Project Structure

```
.
├── client.go          # Client implementations for each provider
├── gemini.go          # Gemini-specific functionality and retry logic
├── model.go           # Model abstraction and parameter handling
├── ollama.go          # Ollama-specific implementations
├── openai.go          # OpenAI-specific implementations
├── provider.go        # Provider abstraction and main interfaces
├── tools/             # Tool implementations (file operations, GitHub, memory, etc.)
├── examples/          # Usage examples for each provider
└── go.mod             # Go module dependencies
```

## Supported Providers

Currently supported:
- **Gemini** (Google)
- **OpenAI** 
- **Ollama**

Planned support:
- **Anthropic**
- **Azure**

## Available Tools

### File Operations
- `readFile` - Read the contents of a file
- `writeFile` - Write content to a file
- `listFiles` - List files in a directory
- `tree` - Display directory tree structure
- `pwd` - Get current working directory

### GitHub Operations
- `get_issues` - Retrieve GitHub issues
- `create_issue` - Create a new GitHub issue
- `update_issue` - Update an existing GitHub issue
- `delete_issue` - Delete a GitHub issue
- `getAssignedIssues` - Get issues assigned to a user
- `getAssignedPRs` - Get pull requests assigned to a user
- And more...

### Memory Operations
- `memory_store` - Store a memory with content and optional metadata
- `memory_retrieve` - Retrieve memories based on semantic similarity to a query
- `memory_update` - Update an existing memory by ID
- `memory_delete` - Delete a memory by ID
- `memory_operation` - Single tool with operation parameter for all memory functions

### Planned Tool Categories
- Slack integration
- Code linting, formatting, and testing tools

## Building and Running

### Prerequisites
- Go 1.23.6 or later
- API keys for the providers you intend to use

### Dependencies
Main dependencies include:
- `github.com/google/generative-ai-go` - Google Gemini SDK
- `github.com/ollama/ollama` - Ollama API client
- `github.com/openai/openai-go` - OpenAI SDK
- `github.com/go-git/go-git/v5` - Git operations
- `github.com/google/go-github/v60` - GitHub API client
- `github.com/lib/pq` - PostgreSQL driver (for Memory Tool)
- `github.com/pgvector/pgvector-go` - Go bindings for pgvector (for Memory Tool)

Install dependencies with:
```bash
go mod tidy
```

### Environment Variables
Set API keys for the providers you're using:
```bash
export GEMINI_API_KEY="your-gemini-api-key"
export OPENAI_API_KEY="your-openai-api-key"
```

For the Memory Tool, you'll also need a PostgreSQL database with the pgvector extension:
```bash
export DATABASE_URL="postgresql://user:password@localhost:5432/database_name"
```

## Usage Examples

### Basic Text Generation
```go
provider, err := genai.NewProvider(genai.GEMINI, genai.ProviderOptions{
    APIKey: os.Getenv("GEMINI_API_KEY"),
})
if err != nil {
    panic(err)
}

response, err := provider.Generate(genai.ModelOptions{
    ModelName: "gemini-2.0-flash-exp",
}, "Hello, world!")
```

### Chat Interface with Tools
```go
tools, err := tools.GetTools([]string{"getAssignedIssues", "pwd", "listFiles"})
if err != nil {
    panic(err)
}

chat := provider.Chat(genai.ModelOptions{
    ModelName: "gemini-2.0-flash-exp",
}, tools)

go func() {
    for msg := range chat.Recv {
        fmt.Println(msg)
        chat.Done <- true
    }
}()

chat.Send <- "What issues am I assigned to?"
<-chat.Done
```

### Memory Tool Usage
```go
// In a real application, you would create an embedding provider and pass it to InitializeMemoryTool
// For example:
// embeddingProvider, err := genai.NewProvider(genai.OPENAI, genai.ProviderOptions{
//     APIKey: os.Getenv("OPENAI_API_KEY"),
// })
// if err != nil {
//     log.Fatal(err)
// }

// Initialize the memory tool
config := tools.MemoryConfig{
    DatabaseURL:       os.Getenv("DATABASE_URL"),
    EmbeddingProvider: "openai",
    EmbeddingModel:    "text-embedding-ada-002",
    EmbeddingDims:     1536,
    DefaultTopK:       5,
}

    // Pass the embedding provider to InitializeMemoryTool
err = tools.InitializeMemoryTool(config, embeddingProvider)
//if err != nil {
//    log.Fatal(err)
//}

// Store a memory
storeArgs := map[string]any{
    "content": "User preference: prefers dark mode UI",
    "metadata": map[string]any{
        "type":    "user_preference",
        "user_id": "12345",
    },
}

storeResult, err := tools.GetTool("memory_store")
if err != nil {
    log.Fatal(err)
}

result, err := storeResult.Run(storeArgs)
if err != nil {
    log.Fatal(err)
}

memoryID := result["id"].(string)
fmt.Printf("Stored memory with ID: %s\n", memoryID)
```

## Development Conventions

### Code Structure
- Each provider has its own implementation file (`gemini.go`, `ollama.go`, etc.)
- Tools are organized by category in the `tools/` directory
- Retry logic is implemented for resilient API calls
- Logging is done through the `logr` interface

### Adding New Providers
1. Create a new provider constant in `provider.go`
2. Implement provider-specific client logic
3. Add provider-specific model handling in `model.go`
4. Implement any provider-specific tool integrations

### Adding New Tools
1. Define the tool in the appropriate category file in `tools/`
2. Register the tool in the `toolMap` in `tools/tool.go`
3. Implement provider-specific tool adapters if needed
4. Add validation and error handling

### Memory Tool
The Memory Tool provides persistent storage and retrieval of contextual information using vector embeddings. It leverages PostgreSQL with the pgvector extension to provide semantic search capabilities across conversation histories, document embeddings, and other contextual data.

Key features:
- **Persistent Storage**: Store text content with associated metadata
- **Semantic Search**: Retrieve memories based on semantic similarity
- **Memory Management**: Update and delete memory entries
- **Configurable Embeddings**: Customizable embedding dimensions and models

The Memory Tool has been refactored to be part of the tools package, eliminating the redundant standalone memory package while maintaining all existing functionality.

When adding new features to the Memory Tool:
1. Extend the `MemoryTool` struct and its methods in `tools/memory.go`
2. Update the tool wrapper functions if new operations are added
3. Ensure proper error handling and context usage
4. Update the example in `examples/memory/main.go` if applicable

## Testing

Run tests with:
```bash
go test ./...
```

Note: Some tests may require valid API keys to run integration tests against actual services.