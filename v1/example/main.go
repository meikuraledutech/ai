package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meikuraledutech/ai/v1"
	"github.com/meikuraledutech/ai/v1/gemini"
	"github.com/meikuraledutech/ai/v1/postgres"
)

type FormResponse struct {
	Nodes []struct {
		Ref  string `json:"ref"`
		Data struct {
			Question string   `json:"question"`
			Type     string   `json:"type"`
			Options  []string `json:"options"`
		} `json:"data"`
	} `json:"nodes"`
	Edges []struct {
		FromNodeRef string `json:"from_node_ref"`
		ToNodeRef   string `json:"to_node_ref"`
		Data        struct {
			Condition string `json:"condition"`
		} `json:"data"`
	} `json:"edges"`
}

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

	// Create store and provider with request logging enabled.
	store := postgres.New(db)
	provider := gemini.New(cfg.GeminiAPI, cfg.ModelID).WithStore(store)

	// Create schema (applies migrations including ai_request_logs table).
	if err := store.CreateSchema(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ Schema created")

	// System prompt with incremental update logic
	systemPrompt := `You are a smart form builder for education. Generate a form structure as a DAG (directed acyclic graph).
Return JSON with "nodes" (questions) and "edges" (conditional paths between questions).
Each node must have: "ref" (temp key), "data" with "question" (text), "type" (radio), and "options" (array of answer choices).
Each edge must have: "from_node_ref", "to_node_ref", "data" with "condition" (the answer/condition that triggers this path).

IMPORTANT - Incremental Updates:
- If you see an existing form in the conversation history, preserve ALL nodes and edges not mentioned in the user's request.
- Only modify, add, or remove nodes/edges that the user explicitly requested.
- Keep all other nodes and edges exactly as they are.
- Maintain referential integrity: all edge references must point to existing nodes.

For new forms:
- Generate at least 3 questions. Maximum 2 nested layers deep.
- Always return the complete form structure.`

	// Start a session with rules.
	session, err := store.CreateSession(ctx, ai.Rules{
		SystemPrompt: systemPrompt,
		MaxTokens:    cfg.MaxTokens,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Session created: %s\n\n", session.ID)

	// PROMPT 1: Create initial form
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("PROMPT 1: Create initial feedback form")
	fmt.Println(strings.Repeat("=", 80))

	prompt1 := "Create a feedback form for a Full Stack Java bootcamp with questions about curriculum, instruction quality, and job readiness."
	fmt.Printf("📝 Prompt: %s\n", prompt1)

	history, _ := store.ListMessages(ctx, session.ID)
	ctxWithSession := context.WithValue(ctx, "session_id", session.ID)
	result1, err := provider.Send(ctxWithSession, session.Rules, history, prompt1)
	if err != nil {
		fmt.Printf("❌ Request failed: %v\n", err)
		return
	}

	fmt.Printf("✓ Response received (%d bytes)\n", len(result1.Content))
	store.AddMessage(ctx, session.ID, "user", prompt1, nil)
	store.AddMessage(ctx, session.ID, "assistant", result1.Content, &result1.Usage)

	// Parse first response
	var form1 FormResponse
	if err := json.Unmarshal([]byte(result1.Content), &form1); err != nil {
		fmt.Printf("❌ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("✓ Form 1: %d nodes, %d edges\n", len(form1.Nodes), len(form1.Edges))
	if len(form1.Nodes) > 0 {
		fmt.Printf("  - First node: %s\n", form1.Nodes[0].Data.Question)
	}

	// PROMPT 2: Update first question based on Form 1
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("PROMPT 2: Update first question")
	fmt.Println(strings.Repeat("=", 80))

	prompt2 := fmt.Sprintf("Change the first question (%s ref) to ask about overall learning experience instead. Keep everything else the same.", form1.Nodes[0].Ref)
	fmt.Printf("📝 Prompt: %s\n", prompt2)

	history, _ = store.ListMessages(ctx, session.ID)
	result2, err := provider.Send(ctxWithSession, session.Rules, history, prompt2)
	if err != nil {
		fmt.Printf("❌ Request failed: %v\n", err)
		return
	}

	fmt.Printf("✓ Response received (%d bytes)\n", len(result2.Content))
	store.AddMessage(ctx, session.ID, "user", prompt2, nil)
	store.AddMessage(ctx, session.ID, "assistant", result2.Content, &result2.Usage)

	// Parse second response
	var form2 FormResponse
	if err := json.Unmarshal([]byte(result2.Content), &form2); err != nil {
		fmt.Printf("❌ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("✓ Form 2: %d nodes, %d edges\n", len(form2.Nodes), len(form2.Edges))
	fmt.Printf("  - First node: %s\n", form2.Nodes[0].Data.Question)

	// PROMPT 3: Add a new question based on Form 2
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("PROMPT 3: Add new question")
	fmt.Println(strings.Repeat("=", 80))

	prompt3 := fmt.Sprintf("Add a new question asking about mentorship quality. Insert it after the current question with ref %s. Keep all other nodes and their conditions exactly as they are.", form2.Nodes[0].Ref)
	fmt.Printf("📝 Prompt: %s\n", prompt3)

	history, _ = store.ListMessages(ctx, session.ID)
	result3, err := provider.Send(ctxWithSession, session.Rules, history, prompt3)
	if err != nil {
		fmt.Printf("❌ Request failed: %v\n", err)
		return
	}

	fmt.Printf("✓ Response received (%d bytes)\n", len(result3.Content))
	store.AddMessage(ctx, session.ID, "user", prompt3, nil)
	store.AddMessage(ctx, session.ID, "assistant", result3.Content, &result3.Usage)

	// Parse third response
	var form3 FormResponse
	if err := json.Unmarshal([]byte(result3.Content), &form3); err != nil {
		fmt.Printf("❌ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("✓ Form 3: %d nodes, %d edges\n", len(form3.Nodes), len(form3.Edges))

	// COMPARISON
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("INCREMENTAL UPDATE ANALYSIS")
	fmt.Println(strings.Repeat("=", 80))

	fmt.Printf("\n📊 Node Count:\n")
	fmt.Printf("  Form 1: %d nodes\n", len(form1.Nodes))
	fmt.Printf("  Form 2: %d nodes (after update, should be same or -1/+1)\n", len(form2.Nodes))
	fmt.Printf("  Form 3: %d nodes (after add, should be +1 from Form 2)\n", len(form3.Nodes))

	fmt.Printf("\n📊 Edge Count:\n")
	fmt.Printf("  Form 1: %d edges\n", len(form1.Edges))
	fmt.Printf("  Form 2: %d edges (should be same or similar)\n", len(form2.Edges))
	fmt.Printf("  Form 3: %d edges (should increase with new connections)\n", len(form3.Edges))

	// Check if Form 1 nodes preserved in Form 2
	fmt.Printf("\n🔍 Structure Preservation Check:\n")
	preservedCount := 0
	for _, n1 := range form1.Nodes {
		for _, n2 := range form2.Nodes {
			if n1.Ref == n2.Ref && n1.Data.Question != n2.Data.Question && n1.Ref == form1.Nodes[0].Ref {
				// First node was updated, that's expected
				continue
			}
			if n1.Ref == n2.Ref && n1.Data.Question == n2.Data.Question {
				preservedCount++
			}
		}
	}
	fmt.Printf("  Form 1 → Form 2: %d/%d nodes preserved (1 should be modified)\n", preservedCount, len(form1.Nodes))

	// Success metrics
	fmt.Printf("\n✅ TEST RESULTS:\n")
	testsPassed := 0
	if len(form3.Nodes) > len(form2.Nodes) {
		fmt.Println("  ✓ Form 3 has more nodes than Form 2 (add worked)")
		testsPassed++
	} else {
		fmt.Println("  ✗ Form 3 should have more nodes than Form 2")
	}

	if len(form2.Nodes) == len(form1.Nodes) || len(form2.Nodes) == len(form1.Nodes)-1 || len(form2.Nodes) == len(form1.Nodes)+1 {
		fmt.Println("  ✓ Form 2 node count reasonable (update only, not regenerate)")
		testsPassed++
	} else {
		fmt.Println("  ✗ Form 2 changed too many nodes (possible full regeneration)")
	}

	if len(form2.Edges) <= len(form1.Edges)+2 {
		fmt.Println("  ✓ Form 2 edges preserved (not excessive regeneration)")
		testsPassed++
	} else {
		fmt.Println("  ✗ Form 2 edges changed too much")
	}

	fmt.Printf("\n📈 Score: %d/3 tests passed\n", testsPassed)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Messages stored in DB for this session")
	fmt.Println(strings.Repeat("=", 80))

	rows, err := db.Query(ctx, `
		SELECT seq, role, LENGTH(content) FROM ai_messages
		WHERE session_id = $1
		ORDER BY seq ASC
	`, session.ID)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var seq int
		var role string
		var contentLen int
		rows.Scan(&seq, &role, &contentLen)
		fmt.Printf("[%d] %s: %d bytes\n", seq, role, contentLen)
	}
}
