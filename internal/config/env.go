package config

import (
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
	return fmt.Errorf("no .env file found")
}

// Retained for potential future multi-file merging needs; currently unused after behavior restoration.
// If later we support layered overrides (.env then .env.local) we can reintroduce a variant using godotenv.Read.
// Keeping stub for clarity and minimal diff footprint.
func loadSingleEnvFile(filename string) error { return godotenv.Load(filename) }
