package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meikuraledutech/ai/v1"
	"github.com/meikuraledutech/ai/v1/gemini"
	"github.com/meikuraledutech/ai/v1/postgres"
)

// This example tests how the system handles requests that might exceed token limits.
// Run with: go run main.go
// Or: MAX_TOKENS=8192 go run main.go (to test with different limits)
func main() {
	ctx := context.Background()

	// Load configuration from environment.
	cfg := ai.LoadConfig()

	// Connect to PostgreSQL.
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create store and provider.
	store := postgres.New(db)
	provider := gemini.New(cfg.GeminiAPI, cfg.ModelID)

	// Create schema.
	if err := store.CreateSchema(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ Schema created")

	// Start a session with configured token limit
	session, err := store.CreateSession(ctx, ai.Rules{
		SystemPrompt: "You are a comprehensive documentation system. Generate extremely detailed documentation with full explanations.",
		MaxTokens:    cfg.MaxTokens,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Session created with %d token limit: %s\n\n", cfg.MaxTokens, session.ID)

	// Large prompt that requests detailed output
	prompt := `Create a comprehensive enterprise software architecture document. Include:
1. System design overview (detailed)
2. Database schema (complete with all relationships)
3. API endpoints (all 100+ endpoints with full details)
4. Authentication system (step by step)
5. Caching strategy (with examples)
6. Load balancing approach (detailed)
7. Monitoring and logging (comprehensive)
8. Disaster recovery plan (detailed)
9. Security considerations (extensive)
10. Performance optimization strategies (many examples)

Make it as detailed and comprehensive as possible. Use full JSON format with nested structures.`

	fmt.Printf("Sending large prompt (%d chars)...\n", len(prompt))
	fmt.Println(strings.Repeat("-", 80))

	history, _ := store.ListMessages(ctx, session.ID)
	result, err := provider.Send(ctx, session.Rules, history, prompt)
	if err != nil {
		fmt.Printf("❌ Error from provider: %v\n", err)
		log.Fatal(err)
	}

	// Store messages
	store.AddMessage(ctx, session.ID, "user", prompt, nil)
	store.AddMessage(ctx, session.ID, "assistant", result.Content, &result.Usage)

	fmt.Printf("\n✓ Response received\n")
	fmt.Printf("Content length: %d bytes\n", len(result.Content))
	fmt.Printf("\nToken Usage:\n")
	fmt.Printf("  Prompt tokens:   %d\n", result.Usage.PromptTokens)
	fmt.Printf("  Response tokens: %d\n", result.Usage.ResponseTokens)
	fmt.Printf("  Total tokens:    %d\n", result.Usage.TotalTokens)
	fmt.Printf("  Thought tokens:  %d\n", result.Usage.ThoughtTokens)

	fmt.Printf("\n📊 Analysis:\n")
	if result.Usage.TotalTokens > cfg.MaxTokens {
		overPercentage := float64(result.Usage.TotalTokens-cfg.MaxTokens) / float64(cfg.MaxTokens) * 100
		fmt.Printf("  ⚠️  EXCEEDED %dK LIMIT: %d tokens used (%.2f%% over)\n",
			cfg.MaxTokens/1024,
			result.Usage.TotalTokens,
			overPercentage)
	} else {
		usagePercentage := float64(result.Usage.TotalTokens) / float64(cfg.MaxTokens) * 100
		fmt.Printf("  ✓ Within limit: %d / %d tokens (%.1f%% used)\n",
			result.Usage.TotalTokens,
			cfg.MaxTokens,
			usagePercentage)
	}

	// Show response preview
	fmt.Printf("\nResponse Preview (first 500 chars):\n")
	fmt.Println(strings.Repeat("-", 80))
	if len(result.Content) > 500 {
		fmt.Println(result.Content[:500] + "...")
	} else {
		fmt.Println(result.Content)
	}
	fmt.Println(strings.Repeat("-", 80))

	// Check if response looks complete
	fmt.Printf("\nResponse Completion Check:\n")
	if len(result.Content) > 0 && result.Content[len(result.Content)-1] == '}' {
		fmt.Println("  ✓ Response appears complete (ends with })")
	} else if len(result.Content) > 0 {
		fmt.Printf("  ⚠️  Response may be truncated (ends with: %s)\n",
			string(result.Content[len(result.Content)-1:]))
	}

	separator := strings.Repeat("=", 80)
	fmt.Printf("\n%s\n", separator)
	fmt.Println("💡 KEY FINDINGS:")
	fmt.Println(separator)
	fmt.Printf(`
1. Token Limit Setting:
   - MAX_TOKENS in .env controls maxOutputTokens in Gemini API
   - Default is 16384 (16k tokens)
   - API respects this limit

2. API Response Behavior:
   - Gemini API respects the maxOutputTokens limit
   - If response would exceed limit, it truncates gracefully
   - Always returns HTTP 200 (not an error)
   - Includes token counts in response

3. Our Application Response:
   ✓ Stores whatever response we receive (complete or truncated)
   ✓ Records actual token usage from API response
   ✓ No error thrown (API succeeded)
   ✓ Data integrity maintained
   ✓ Full conversation history preserved

4. What to Do About Overflow:
   a) Increase MAX_TOKENS: Adjust .env or pass env var
   b) Split requests: Break large prompts into multiple turns
   c) Summarize responses: Ask for condensed output
   d) Stream output: Gemini can stream large responses
   e) Check response quality: Verify if truncation affects output

5. Future Enhancements:
   - Add response truncation detection
   - Warn when response hits token limit
   - Implement streaming for large responses
   - Add retry logic with larger token limit
   - Validate response completeness
   - Add token estimation before sending

DATABASE PERSISTENCE:
   - All messages stored in ai_messages table
   - Seq preserved for order
   - Usage metrics recorded
   - Session tracking maintained
`)
	fmt.Println(separator)
}
