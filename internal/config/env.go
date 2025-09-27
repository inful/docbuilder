package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// loadEnvFile loads environment variables honoring precedence:
// 1. Existing process env (never overwritten)
// 2. .env (base)
// 3. .env.local (overrides .env if present)
// Returns nil if neither file exists (non-fatal for callers).
func loadEnvFile() error {
	loadedAny := false
	if _, err := os.Stat(".env"); err == nil {
		if err := loadSingleEnvFile(".env"); err != nil {
			return fmt.Errorf("failed parsing .env: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Loaded environment variables from .env\n")
		loadedAny = true
	}
	if _, err := os.Stat(".env.local"); err == nil {
		if err := loadSingleEnvFile(".env.local"); err != nil {
			return fmt.Errorf("failed parsing .env.local: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Loaded environment variables from .env.local (overrides)\n")
		loadedAny = true
	}
	if !loadedAny {
		return fmt.Errorf("no .env file found")
	}
	return nil
}

// loadSingleEnvFile parses a single env file using godotenv while respecting existing process vars.
func loadSingleEnvFile(filename string) error {
	values, err := godotenv.Read(filename)
	if err != nil {
		return err
	}
	for k, v := range values {
		if os.Getenv(k) == "" { // never override existing process env
			_ = os.Setenv(k, v)
		}
	}
	return nil
}
