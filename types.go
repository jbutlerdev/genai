package tools

import "github.com/google/generative-ai-go/genai"

const (
	// debug option for verbose output
	DEBUG = false

	GEMINI    = "gemini"
	ANTHROPIC = "anthropic"
	OPENAI    = "openai"
	OLLAMA    = "ollama"
)

type GenAIProvider struct {
	Type string
}

type RunnableTool struct {
	GeminiTool *genai.Tool
}

func NewGenAIProvider(provider string) *GenAIProvider {
	return &GenAIProvider{
		Type: provider,
	}
}

func (g *GenAIProvider) RunTool(toolName string, args map[string]any) (any, error) {
	switch g.Type {
	case GEMINI:
		return runGeminiTool(toolName, args)
	}
	return nil, nil
}

func (g *GenAIProvider) GetTool(toolName string) (*RunnableTool, error) {
	switch g.Type {
	case GEMINI:
		geminiTool, err := getGeminiTool(toolName)
		if err != nil {
			return nil, err
		}
		return &RunnableTool{GeminiTool: geminiTool}, nil
	}
	return nil, nil
}
