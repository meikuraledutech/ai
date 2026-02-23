package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/meikuraledutech/ai/v1"
)

// AddRequestLog inserts a new request log with pending status.
func (s *PGStore) AddRequestLog(ctx context.Context, log ai.RequestLog) (*ai.RequestLog, error) {
	id := uuid.New().String()
	now := time.Now()

	err := s.db.QueryRow(ctx, `
		INSERT INTO ai_request_logs (
			id, session_id, prompt, response, attempt_number,
			retry_count, final_status, fail_reason, error_message,
			prompt_tokens, response_tokens, total_tokens, thought_tokens,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING created_at, updated_at
	`,
		id, log.SessionID, log.Prompt, log.Response, log.AttemptNumber,
		log.RetryCount, ai.StatusPending, "", "",
		0, 0, 0, 0,
		now, now,
	).Scan(&log.CreatedAt, &log.UpdatedAt)

	if err != nil {
		return nil, err
	}

	log.ID = id
	log.FinalStatus = ai.StatusPending
	return &log, nil
}

// UpdateRequestLog updates an existing request log with completion/retry details.
func (s *PGStore) UpdateRequestLog(ctx context.Context, id string, response string, status string, failReason string, errorMsg string, retryCount int, usage *ai.Usage) error {
	promptTokens := 0
	responseTokens := 0
	totalTokens := 0
	thoughtTokens := 0

	if usage != nil {
		promptTokens = usage.PromptTokens
		responseTokens = usage.ResponseTokens
		totalTokens = usage.TotalTokens
		thoughtTokens = usage.ThoughtTokens
	}

	_, err := s.db.Exec(ctx, `
		UPDATE ai_request_logs
		SET
			response = $1,
			final_status = $2,
			fail_reason = $3,
			error_message = $4,
			retry_count = $5,
			prompt_tokens = $6,
			response_tokens = $7,
			total_tokens = $8,
			thought_tokens = $9,
			updated_at = NOW()
		WHERE id = $10
	`,
		response, status, failReason, errorMsg, retryCount,
		promptTokens, responseTokens, totalTokens, thoughtTokens,
		id,
	)

	return err
}
