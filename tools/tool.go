package tools

import (
	"fmt"

	"github.com/google/generative-ai-go/genai"
	ollama "github.com/ollama/ollama/api"
)

const DEBUG = false

type Tool struct {
	Name        string
	Description string
	Parameters  []Parameter
	Options     map[string]string
	Run         func(map[string]any) (map[string]any, error)
	Summarize   bool
}

type RunnableTool struct {
	GeminiTool *genai.Tool
	OllamaTool *ollama.Tool
}

type Parameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

var toolMap = mergeTools(fileTools, githubTools, gitTools, searchTools, memoryTools)

func mergeTools(tools ...map[string]Tool) map[string]Tool {
	keys := make(map[string]bool)
	merged := make(map[string]Tool)
	for _, tool := range tools {
		for key, value := range tool {
			if _, ok := keys[key]; ok {
				panic(fmt.Sprintf("duplicate tool name: %s", key))
			}
			keys[key] = true
			merged[key] = value
		}
	}
	return merged
}

func GetTool(toolName string) (*Tool, error) {
	tool, ok := toolMap[toolName]
	if !ok {
		return nil, fmt.Errorf("tool %s does not exist", toolName)
	}
	return &tool, nil
}

func GetTools(toolNames []string) ([]*Tool, error) {
	tools := make([]*Tool, len(toolNames))
	for i, toolName := range toolNames {
		tool, err := GetTool(toolName)
		if err != nil {
			return nil, err
		}
		tools[i] = tool
	}
	return tools, nil
}

func Tools() []string {
	tools := make([]string, 0, len(toolMap))
	for toolName := range toolMap {
		tools = append(tools, toolName)
	}
	return tools
}
