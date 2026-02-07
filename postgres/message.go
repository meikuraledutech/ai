package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/meikuraledutech/ai"
)

// AddMessage appends a message to a session with auto-incremented seq.
func (s *PGStore) AddMessage(ctx context.Context, sessionID string, role string, content string, usage *ai.Usage) (*ai.Message, error) {
	msg := &ai.Message{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Usage:     usage,
	}

	var promptTokens, responseTokens, totalTokens, thoughtTokens int
	if usage != nil {
		promptTokens = usage.PromptTokens
		responseTokens = usage.ResponseTokens
		totalTokens = usage.TotalTokens
		thoughtTokens = usage.ThoughtTokens
	}

	err := s.db.QueryRow(ctx,
		`INSERT INTO ai_messages (id, session_id, seq, role, content, prompt_tokens, response_tokens, total_tokens, thought_tokens)
		 VALUES ($1, $2, COALESCE((SELECT MAX(seq) FROM ai_messages WHERE session_id = $2), 0) + 1, $3, $4, $5, $6, $7, $8)
		 RETURNING seq, created_at`,
		msg.ID, sessionID, role, content, promptTokens, responseTokens, totalTokens, thoughtTokens,
	).Scan(&msg.Seq, &msg.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("ai: add message: %w", err)
	}

	return msg, nil
}

// ListMessages returns all messages for a session ordered by seq.
func (s *PGStore) ListMessages(ctx context.Context, sessionID string) ([]ai.Message, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, session_id, seq, role, content, prompt_tokens, response_tokens, total_tokens, thought_tokens, created_at
		 FROM ai_messages WHERE session_id = $1 ORDER BY seq ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("ai: list messages: %w", err)
	}
	defer rows.Close()

	var messages []ai.Message
	for rows.Next() {
		var msg ai.Message
		var pt, rt, tt, tht int

		err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Seq, &msg.Role, &msg.Content, &pt, &rt, &tt, &tht, &msg.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("ai: scan message: %w", err)
		}

		if pt > 0 || rt > 0 || tt > 0 || tht > 0 {
			msg.Usage = &ai.Usage{
				PromptTokens:   pt,
				ResponseTokens: rt,
				TotalTokens:    tt,
				ThoughtTokens:  tht,
			}
		}

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ai: list messages: %w", err)
	}

	return messages, nil
}

// Ensure PGStore implements ai.Store at compile time.
var _ ai.Store = (*PGStore)(nil)
