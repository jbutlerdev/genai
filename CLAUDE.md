# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go library that provides a unified wrapper for multiple Generative AI providers (Gemini, OpenAI, Ollama, Anthropic). It includes an extensible tool system allowing AI models to interact with files, GitHub, Git, and search functionality.

## Key Commands

### Build and Development
```bash
# Download dependencies
go mod download

# Tidy dependencies
go mod tidy

# Build the entire project
go build ./...

# Run examples (set appropriate API keys first)
export GEMINI_API_KEY="your-key"
cd examples/gemini
go run getOpenIssues.go

# Run linting
golangci-lint run ./...
```

### Testing
Currently, there are no unit tests in the project. When adding tests:
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestName ./path/to/package
```

## Architecture Overview

### Provider System
The library uses an abstraction layer to support multiple AI providers:
- **Provider Interface** (`provider.go`): Defines common methods all providers must implement
- **Implementations**: `gemini.go`, `openai.go`, `ollama.go` 
- **Model Options** (`model.go`): Unified configuration for model parameters

### Tool Framework
Located in `tools/` directory:
- **Base Definition** (`tool.go`): Core tool interfaces and structures
- **Provider-Specific**: `gemini.go`, `ollama.go` for provider-specific tool implementations
- **Tool Categories**: File operations, GitHub operations, Git operations, Search

### Chat System
- Uses Go channels for asynchronous communication
- Pattern: Send channel for input, Recv channel for output, Done channel for cleanup
- GenerationComplete channel signals when a response is finished

### Key Patterns

1. **Provider Initialization**:
   ```go
   provider, err := genai.NewProvider(genai.GEMINI, genai.ProviderOptions{
       APIKey: os.Getenv("GEMINI_API_KEY"),
   })
   ```

2. **Chat with Tools**:
   ```go
   tools, _ := tools.GetTools([]string{"github", "file"})
   chat := provider.Chat(modelOptions, tools)
   ```

3. **Error Handling**: The library returns errors following Go conventions. Always check returned errors.

## Important Notes

- **Environment Variables**: Each provider requires specific environment variables (GEMINI_API_KEY, OPENAI_API_KEY, OLLAMA_BASE_URL)
- **Model Options**: Recent changes require using `ModelOptions` struct instead of string for model configuration
- **No Tests**: The project currently lacks unit tests - consider adding tests when implementing new features
- **Examples**: Some examples may have outdated model parameter usage (string vs ModelOptions)