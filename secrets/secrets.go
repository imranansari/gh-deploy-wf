package secrets

import (
	"fmt"
	"os"
)

// LoadFromFile loads a secret from a file path
func LoadFromFile(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("secret file path is empty")
	}
	
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret file %s: %w", path, err)
	}
	
	if len(data) == 0 {
		return nil, fmt.Errorf("secret file %s is empty", path)
	}
	
	return data, nil
}

// GetSecretPath returns the secret file path from environment variable
// or falls back to default path if not set
func GetSecretPath(envVar, defaultPath string) string {
	if path := os.Getenv(envVar); path != "" {
		return path
	}
	return defaultPath
}