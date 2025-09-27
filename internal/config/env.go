package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// loadEnvFile loads environment variables from .env/.env.local files (shared with v2 loader).
// It attempts each supported filename in order and stops at the first successfully parsed file.
// This preserves existing semantics while isolating environment loading concerns.
func loadEnvFile() error {
	envPaths := []string{".env", ".env.local"}
	for _, envPath := range envPaths {
		if err := loadSingleEnvFile(envPath); err == nil {
			fmt.Fprintf(os.Stderr, "Loaded environment variables from %s\n", envPath)
			return nil
		}
	}
	return fmt.Errorf("no .env file found")
}

// loadSingleEnvFile loads environment variables from a single file in KEY=VALUE format.
// Existing process environment variables are not overwritten.
func loadSingleEnvFile(filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return err
	}
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if os.Getenv(key) == "" { // do not override existing env
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
