package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// loadEnvFile loads environment variables using the first existing file among
// .env then .env.local. This preserves the original behavior (only one file used)
// while delegating parsing to godotenv. Existing process environment variables
// are never overwritten (godotenv.Load semantics).
func loadEnvFile() error {
	envPaths := []string{".env", ".env.local"}
	for _, p := range envPaths {
		if _, err := os.Stat(p); err == nil {
			if err := godotenv.Load(p); err != nil {
				return fmt.Errorf("failed loading %s: %w", p, err)
			}
			fmt.Fprintf(os.Stderr, "Loaded environment variables from %s\n", p)
			return nil
		}
	}
	return errors.New("no .env file found")
}
