package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/meikuraledutech/ai"
)

// CreateSession creates a new session with the given rules.
func (s *PGStore) CreateSession(ctx context.Context, rules ai.Rules) (*ai.Session, error) {
	session := &ai.Session{
		ID:    uuid.New().String(),
		Rules: rules,
	}

	err := s.db.QueryRow(ctx,
		`INSERT INTO ai_sessions (id, system_prompt, output_schema, max_tokens)
		 VALUES ($1, $2, $3, $4)
		 RETURNING created_at`,
		session.ID, rules.SystemPrompt, rules.OutputSchema, rules.MaxTokens,
	).Scan(&session.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("ai: create session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session by ID.
func (s *PGStore) GetSession(ctx context.Context, sessionID string) (*ai.Session, error) {
	session := &ai.Session{ID: sessionID}

	err := s.db.QueryRow(ctx,
		`SELECT system_prompt, output_schema, max_tokens, created_at
		 FROM ai_sessions WHERE id = $1`,
		sessionID,
	).Scan(&session.Rules.SystemPrompt, &session.Rules.OutputSchema, &session.Rules.MaxTokens, &session.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("ai: get session: %w", err)
	}

	return session, nil
}
