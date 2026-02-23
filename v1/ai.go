package ai

import "time"

// Rules control AI behavior per request.
type Rules struct {
	SystemPrompt string `json:"system_prompt"`
	OutputSchema string `json:"output_schema"`
	MaxTokens    int    `json:"max_tokens"`
}

// Usage holds token counts from the AI provider response.
type Usage struct {
	PromptTokens   int `json:"prompt_tokens"`
	ResponseTokens int `json:"response_tokens"`
	TotalTokens    int `json:"total_tokens"`
	ThoughtTokens  int `json:"thought_tokens"`
}

// Message is a single turn in a conversation.
type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Seq       int       `json:"seq"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Usage     *Usage    `json:"usage,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Session groups messages into a conversation.
type Session struct {
	ID        string    `json:"id"`
	Rules     Rules     `json:"rules"`
	CreatedAt time.Time `json:"created_at"`
}

// Result is what the provider returns — content + token usage.
type Result struct {
	Content string `json:"content"`
	Usage   Usage  `json:"usage"`
}

// Config holds defaults for the ai package.
type Config struct {
	DefaultMaxTokens int
}

// MigrationRecord tracks a single applied migration.
type MigrationRecord struct {
	Name      string
	Applied   bool
	AppliedAt *time.Time
	Checksum  string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		DefaultMaxTokens: 16384,
	}
}
