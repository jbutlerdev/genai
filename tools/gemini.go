package tools

import (
	"fmt"

	"github.com/google/generative-ai-go/genai"
)

func RunGeminiTool(toolName string, args map[string]any) (any, error) {
	tool, ok := toolMap[toolName]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
	resp, err := tool.Run(args)
	if err != nil {
		return nil, fmt.Errorf("failed to run tool: %v", err)
	}
	return genai.FunctionResponse{
		Name:     toolName,
		Response: resp,
	}, nil
}

func GetGeminiTool(name string) (*genai.Tool, error) {
	tool, ok := toolMap[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	var params *genai.Schema
	if len(tool.Parameters) > 0 {
		params = &genai.Schema{
			Type:       genai.TypeObject,
			Properties: make(map[string]*genai.Schema),
			Required:   make([]string, 0),
		}
		for _, param := range tool.Parameters {
			genaiParam := paramToGenaiSchema(param)
			if genaiParam == nil {
				return nil, fmt.Errorf("unknown parameter type: %s", param.Type)
			}
			params.Properties[param.Name] = genaiParam
			if param.Required {
				params.Required = append(params.Required, param.Name)
			}
		}
	}

	geminiTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  params,
		}},
	}
	if DEBUG {
		printGeminiTool(geminiTool)
	}
	return geminiTool, nil
}

func paramToGenaiSchema(param Parameter) *genai.Schema {
	switch param.Type {
	case "string":
		return &genai.Schema{
			Type:        genai.TypeString,
			Description: param.Description,
		}
	case "stringArray":
		return &genai.Schema{
			Type:        genai.TypeArray,
			Description: param.Description,
			Items: &genai.Schema{
				Type: genai.TypeString,
			},
		}
	case "boolean":
		return &genai.Schema{
			Type:        genai.TypeBoolean,
			Description: param.Description,
		}
	}
	return nil
}

func printGeminiTool(tool *genai.Tool) {
	fmt.Printf("Name: %s\n", tool.FunctionDeclarations[0].Name)
	fmt.Printf("Description: %s\n", tool.FunctionDeclarations[0].Description)
	fmt.Printf("Parameters: \n")
	if tool.FunctionDeclarations[0].Parameters != nil {
		for key, param := range tool.FunctionDeclarations[0].Parameters.Properties {
			fmt.Printf("Name: %s\n", key)
			fmt.Printf("  Type: %s\n", param.Type)
			fmt.Printf("  Description: %s\n", param.Description)
			fmt.Printf("  Items: %+v\n", param.Items)
		}
		fmt.Printf("Required: %v\n", tool.FunctionDeclarations[0].Parameters.Required)
	}
}
