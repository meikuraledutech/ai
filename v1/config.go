package ai

import (
	"os"
	"strconv"
)

// Config holds configuration for the AI package loaded from environment variables.
type AppConfig struct {
	DatabaseURL string
	GeminiAPI   string
	ModelID     string
	MaxTokens   int
}

// LoadConfig loads configuration from environment variables with sensible defaults.
func LoadConfig() AppConfig {
	cfg := AppConfig{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		GeminiAPI:   os.Getenv("GEMINI_API"),
		ModelID:     os.Getenv("MODEL_ID"),
		MaxTokens:   16384, // default
	}

	// Parse MAX_TOKENS if provided
	if mt := os.Getenv("MAX_TOKENS"); mt != "" {
		if parsed, err := strconv.Atoi(mt); err == nil && parsed > 0 {
			cfg.MaxTokens = parsed
		}
	}

	return cfg
}
