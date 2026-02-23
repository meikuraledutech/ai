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

func main() {
	ctx := context.Background()

	// Load configuration from environment.
	cfg := ai.LoadConfig()

	// Override token limit to 2000 for testing validator with truncation
	cfg.MaxTokens = 2000

	// Connect to PostgreSQL.
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create store and provider with request logging enabled.
	store := postgres.New(db)
	provider := gemini.New(cfg.GeminiAPI, cfg.ModelID).WithStore(store)

	// Create schema (applies migrations including ai_request_logs table).
	if err := store.CreateSchema(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ Schema created")

	// Start a session with rules.
	session, err := store.CreateSession(ctx, ai.Rules{
		SystemPrompt: "You are a form builder. Return JSON with nodes and edges for the requested form.",
		MaxTokens:    cfg.MaxTokens,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Session created: %s\n", session.ID)
	fmt.Printf("✓ Token limit set to: %d (low limit to trigger validation/retry)\n\n", cfg.MaxTokens)

	// First turn - request that will likely exceed token limit and trigger validator.
	prompt := "Create a comprehensive registration form with the following fields: first name, last name, email, phone, street address, city, state, zip code, country, date of birth, gender, company name, job title, department, industry, company size, years of experience, education level, linkedin profile, website. Return the form as complete, valid JSON with all fields."
	fmt.Printf("Sending prompt (%d chars)...\n", len(prompt))
	fmt.Println("---")

	history, _ := store.ListMessages(ctx, session.ID)
	// Pass sessionID via context so provider can log requests
	ctxWithSession := context.WithValue(ctx, "session_id", session.ID)
	result, err := provider.Send(ctxWithSession, session.Rules, history, prompt)
	if err != nil {
		fmt.Printf("⚠️  Request failed: %v\n\n", err)
	} else {
		fmt.Printf("✓ Response received (%d bytes)\n", len(result.Content))
		store.AddMessage(ctx, session.ID, "user", prompt, nil)
		store.AddMessage(ctx, session.ID, "assistant", result.Content, &result.Usage)
		fmt.Printf("✓ Messages stored\n")
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("REQUEST LOGS (from ai_request_logs table)")
	fmt.Println(strings.Repeat("=", 80))

	// Query request logs to show what was tracked
	rows, err := db.Query(ctx, `
		SELECT id, final_status, fail_reason, error_message, attempt_number, retry_count,
		       prompt_tokens, response_tokens, total_tokens, created_at
		FROM ai_request_logs
		WHERE session_id = $1
		ORDER BY created_at ASC
	`, session.ID)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		var id, status, failReason, errMsg string
		var attempt, retries, promptTok, respTok, totalTok int
		var createdAt interface{}

		if err := rows.Scan(&id, &status, &failReason, &errMsg, &attempt, &retries,
			&promptTok, &respTok, &totalTok, &createdAt); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("\n[Log #%d]\n", count)
		fmt.Printf("  ID: %s\n", id[:8]+"...")
		fmt.Printf("  Status: %s\n", status)
		if failReason != "" {
			fmt.Printf("  Fail Reason: %s\n", failReason)
		}
		if errMsg != "" {
			fmt.Printf("  Error: %s\n", errMsg)
		}
		fmt.Printf("  Attempt #%d (retries: %d)\n", attempt, retries)
		fmt.Printf("  Tokens: prompt=%d response=%d total=%d\n", promptTok, respTok, totalTok)
	}

	if count == 0 {
		fmt.Println("❌ No request logs found - logging may not be working!")
	} else {
		fmt.Printf("\n✓ Total logs recorded: %d\n", count)
	}

	fmt.Println(strings.Repeat("=", 80) + "\n")
}
