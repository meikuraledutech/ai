package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meikuraledutech/ai/v1"
	"github.com/meikuraledutech/ai/v1/gemini"
	"github.com/meikuraledutech/ai/v1/postgres"
)

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
	fmt.Println("Schema created.")

	// Start a session with rules.
	session, err := store.CreateSession(ctx, ai.Rules{
		SystemPrompt: "You are a form builder. Return JSON with nodes and edges for the requested form.",
		MaxTokens:    cfg.MaxTokens,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Session created: %s\n", session.ID)

	// First turn.
	prompt := "Create a simple registration form with name, email, and phone fields."
	history, _ := store.ListMessages(ctx, session.ID)
	result, err := provider.Send(ctx, session.Rules, history, prompt)
	if err != nil {
		log.Fatal(err)
	}

	store.AddMessage(ctx, session.ID, "user", prompt, nil)
	store.AddMessage(ctx, session.ID, "assistant", result.Content, &result.Usage)

	fmt.Printf("Response: %s\n", result.Content)
	fmt.Printf("Usage: prompt=%d response=%d total=%d thought=%d\n",
		result.Usage.PromptTokens, result.Usage.ResponseTokens, result.Usage.TotalTokens, result.Usage.ThoughtTokens)

	// Second turn.
	prompt2 := "Add an address field after the phone field."
	history, _ = store.ListMessages(ctx, session.ID)
	result2, err := provider.Send(ctx, session.Rules, history, prompt2)
	if err != nil {
		log.Fatal(err)
	}

	store.AddMessage(ctx, session.ID, "user", prompt2, nil)
	store.AddMessage(ctx, session.ID, "assistant", result2.Content, &result2.Usage)

	fmt.Printf("Response 2: %s\n", result2.Content)
	fmt.Printf("Usage 2: prompt=%d response=%d total=%d thought=%d\n",
		result2.Usage.PromptTokens, result2.Usage.ResponseTokens, result2.Usage.TotalTokens, result2.Usage.ThoughtTokens)
}
