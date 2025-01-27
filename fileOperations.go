package tools

import (
	"fmt"
	"os"
)

var fileTools = map[string]Tool{
	"tree":      treeTool,
	"pwd":       pwdTool,
	"writeFile": writeFileTool,
}

var pwdTool = Tool{
	Name:        "pwd",
	Description: "Get the current working directory",
	Parameters:  nil,
	Run:         PWD,
}

func PWD(_ map[string]any) (map[string]any, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	return map[string]any{
		"path": pwd,
	}, nil
}

func ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(content), nil
}

var writeFileTool = Tool{
	Name:        "writeFile",
	Description: "Write to a file",
	Parameters: []Parameter{
		{
			Name:        "path",
			Type:        "string",
			Description: "The path to the file to write to",
			Required:    true,
		},
		{
			Name:        "content",
			Type:        "string",
			Description: "The content to write to the file",
			Required:    true,
		},
		{
			Name:        "executable",
			Type:        "boolean",
			Description: "Whether the file should be executable",
			Required:    false,
		},
	},
	Run: WriteFile,
}

func WriteFile(args map[string]any) (map[string]any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("expected string: %v", args["path"])
	}
	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("expected string: %v", args["content"])
	}
	executable, ok := args["executable"].(bool)
	if !ok {
		executable = false
	}
	mode := os.FileMode(0644)
	if executable {
		mode = os.FileMode(0755)
	}
	// end content with newline if it doesn't already end with one
	if content[len(content)-1] != '\n' {
		content += "\n"
	}
	err := os.WriteFile(path, []byte(content), mode)
	if err != nil {
		return map[string]any{
			"success": false,
		}, fmt.Errorf("failed to write file: %w", err)
	}
	return map[string]any{
		"success": true,
	}, nil
}

func DeleteFile(path string) error {
	return os.Remove(path)
}

func RenameFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func ListFiles(path string) ([]string, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = file.Name()
	}
	return names, nil
}

func ListDirectories(path string) ([]string, error) {
	dirs, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list directories: %w", err)
	}
	names := make([]string, len(dirs))
	for i, dir := range dirs {
		if dir.IsDir() {
			names[i] = dir.Name()
		}
	}
	return names, nil
}

var treeTool = Tool{
	Name:        "tree",
	Description: "List the files and directories in a given path",
	Parameters: []Parameter{
		{
			Name:        "path",
			Type:        "string",
			Description: "The path to the directory to list",
			Required:    true,
		},
		{
			Name:        "exclude",
			Type:        "stringArray",
			Description: "The directories to exclude from the list",
			Required:    false,
		},
	},
	Run: Tree,
}

func Tree(args map[string]any) (map[string]any, error) {
	var output string
	root, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("expected string: %v", args["path"])
	}
	excludeList, ok := args["exclude"].([]string)
	if !ok {
		excludeList = []string{".git"}
	}

	// Start with the root directory name
	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("failed to access root path: %w", err)
	}
	output = rootInfo.Name() + "\n"

	// Walk the directory tree
	subTree, err := subTree(root, "", excludeList)
	if err != nil {
		return nil, err
	}
	output += subTree

	return map[string]any{
		"path": output,
	}, nil
}

func subTree(path string, prefix string, excludeList []string) (string, error) {
	var output string
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", path, err)
	}

	for i, entry := range entries {
		// Check if the entry should be excluded
		shouldExclude := false
		for _, exclude := range excludeList {
			if entry.Name() == exclude {
				shouldExclude = true
				break
			}
		}
		if shouldExclude {
			continue
		}

		// Create the appropriate prefix for this item
		isLast := i == len(entries)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		// Add this item to the output
		output += prefix + connector + entry.Name() + "\n"
		// If it's a directory, recursively process its contents
		if entry.IsDir() {
			newPrefix := prefix
			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			subTree, err := subTree(path+"/"+entry.Name(), newPrefix, excludeList)
			if err != nil {
				return "", err
			}
			output += subTree
		}
	}
	return output, nil
}
