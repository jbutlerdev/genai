package tools

import "fmt"

type Tool struct {
	Name        string
	Description string
	Parameters  []Parameter
	Run         func(map[string]any) (map[string]any, error)
}

type Parameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

var toolMap = mergeTools(fileTools, githubTools)

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
