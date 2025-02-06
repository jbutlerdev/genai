package tools

import (
	"encoding/json"
	"fmt"

	ollama "github.com/ollama/ollama/api"
)

type OllamaFunctionProperties struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

func RunOllamaTool(toolName string, args map[string]any) (any, error) {
	tool, ok := toolMap[toolName]
	if !ok {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("unknown tool: %s", toolName),
		}, fmt.Errorf("unknown tool: %s", toolName)
	}
	resp, err := tool.Run(args)
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return ollama.Message{
		Role:    "tool",
		Content: string(jsonResp),
	}, err
}

func GetOllamaTool(name string) (*ollama.Tool, error) {
	tool, ok := toolMap[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	ollamaTool := &ollama.Tool{
		Function: ollama.ToolFunction{
			Name:        tool.Name,
			Description: tool.Description,
		},
	}
	if len(tool.Parameters) > 0 {
		ollamaTool.Function.Parameters.Type = "object"
		ollamaTool.Function.Parameters.Properties = make(map[string]struct {
			Type        string   `json:"type"`
			Description string   `json:"description"`
			Enum        []string `json:"enum,omitempty"`
		})
		ollamaTool.Function.Parameters.Required = make([]string, 0)
		for _, param := range tool.Parameters {
			ollamaParam := paramToOllamaFunctionProperties(param)
			ollamaTool.Function.Parameters.Properties[param.Name] = ollamaParam
			if param.Required {
				ollamaTool.Function.Parameters.Required = append(ollamaTool.Function.Parameters.Required, param.Name)
			}
		}
	}

	if DEBUG {
		printOllamaTool(ollamaTool)
	}
	return ollamaTool, nil
}

func paramToOllamaFunctionProperties(param Parameter) OllamaFunctionProperties {
	switch param.Type {
	case "string":
		return OllamaFunctionProperties{
			Type:        "string",
			Description: param.Description,
		}
	case "stringArray":
		return OllamaFunctionProperties{
			Type:        "string[]",
			Description: param.Description,
		}
	}
	return OllamaFunctionProperties{}
}

func printOllamaTool(tool *ollama.Tool) {
	fmt.Printf("Name: %s\n", tool.Function.Name)
	fmt.Printf("Description: %s\n", tool.Function.Description)
	fmt.Printf("Parameters: \n")
	for key, param := range tool.Function.Parameters.Properties {
		fmt.Printf("Name: %s\n", key)
		fmt.Printf("  Type: %s\n", param.Type)
		fmt.Printf("  Description: %s\n", param.Description)
	}
}
