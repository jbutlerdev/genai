package main

import (
	"bufio"
	"os"
	"strings"
)

// readConfig reads the YAML configuration file and extracts provider and model
func readConfig() (provider, model string) {
	configPath := "/data/jbutler/git/jbutlerdev/genai/examples/memory/sample-config.yml"
	file, err := os.Open(configPath)
	if err != nil {
		fmt.Printf("DEBUG: Failed to open config file: %v\n", err)
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
	
	fmt.Printf("DEBUG: Config file read - provider: '%s', model: '%s'\n", provider, model)

	return provider, model
}