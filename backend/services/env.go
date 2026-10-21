package services

import (
	"log"
	"os"
	"strings"
)

// envFiles are searched in order; a variable already present in the environment is never overridden.
var envFiles = []string{".env.local", ".env", "../.env"}

// LoadEnvVariables loads KEY=VALUE pairs from local dotenv files into the process environment.
// It is a no-op in deployed environments where no dotenv files exist.
func LoadEnvVariables() {
	for _, path := range envFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		log.Printf("Loading environment from %s", path)
		applyEnvFile(string(content))
	}
}

func applyEnvFile(content string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		os.Setenv(key, strings.TrimSpace(value))
	}
}
