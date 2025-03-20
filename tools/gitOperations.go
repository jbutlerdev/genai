package tools

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/go-git/go-git/v5"
)

var gitTools = map[string]Tool{
	"applyPatch": applyPatchTool,
	"revertFile": revertFileTool,
}

var applyPatchTool = Tool{
	Name:        "applyPatch",
	Description: "Apply a patch to the current repository",
	Parameters: []Parameter{
		{
			Name:        "patch",
			Type:        "string",
			Description: "The patch to apply",
		},
	},
	Options: map[string]string{},
	Run:     ApplyPatch,
}

func ApplyPatch(args map[string]any) (map[string]any, error) {
	patch, ok := args["patch"].(string)
	if !ok {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("expected string: %v", args["patch"]),
		}, fmt.Errorf("expected string: %v", args["patch"])
	}
	path, ok := args["basePath"].(string)
	if !ok {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("expected string: %v", args["basePath"]),
		}, fmt.Errorf("expected to be provided a path: %v", args["basePath"])
	}
	repo, err := git.PlainOpen(path)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to open repository: %v", err),
		}, fmt.Errorf("failed to open repository: %v", err)
	}
	patchBytes := []byte(patch)

	// Create a temporary file for the patch
	tmpFile, err := os.CreateTemp("", "git-patch-*.patch")
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to create temp file: %v", err),
		}, fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write the patch to the temp file
	if _, err := tmpFile.Write(patchBytes); err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to write patch to temp file: %v", err),
		}, fmt.Errorf("failed to write patch to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to close temp file: %v", err),
		}, fmt.Errorf("failed to close temp file: %v", err)
	}

	// Get the repository root directory
	wt, err := repo.Worktree()
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to get worktree: %v", err),
		}, fmt.Errorf("failed to get worktree: %v", err)
	}

	// Execute git apply command
	cmd := exec.Command("git", "apply", tmpFile.Name())
	cmd.Dir = wt.Filesystem.Root()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to apply patch: %v, output: %s", err, output),
		}, fmt.Errorf("failed to apply patch: %v, output: %s", err, output)
	}

	return map[string]any{
		"success": true,
	}, nil
}

var revertFileTool = Tool{
	Name:        "revertFile",
	Description: "Revert a file to the previous commit",
	Parameters: []Parameter{
		{
			Name:        "file",
			Type:        "string",
			Description: "The file to revert",
		},
	},
	Options: map[string]string{},
	Run:     RevertFileWrapper,
}

func RevertFileWrapper(args map[string]any) (map[string]any, error) {
	path, ok := args["basePath"].(string)
	if !ok {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("expected string: %v", args["basePath"]),
		}, fmt.Errorf("expected to be provided a path: %v", args["basePath"])
	}

	file, ok := args["file"].(string)
	if !ok {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("expected string: %v", args["file"]),
		}, fmt.Errorf("expected to be provided a file: %v", args["file"])
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to open repository: %v", err),
		}, fmt.Errorf("failed to open repository: %v", err)
	}

	err = RevertFile(repo, file)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to revert file: %v", err),
		}, fmt.Errorf("failed to revert file: %v", err)
	}

	return map[string]any{
		"success": true,
	}, nil
}

func RevertFile(repo *git.Repository, file string) error {
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %v", err)
	}

	err = wt.Filesystem.Remove(file)
	if err != nil {
		return fmt.Errorf("failed to remove file: %v", err)
	}

	return nil
}
