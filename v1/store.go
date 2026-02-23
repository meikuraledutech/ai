package ai

import (
	"context"
	"errors"
)

var (
	ErrSessionNotFound = errors.New("ai: session not found")
)

// Store defines the contract for persisting sessions and messages.
type Store interface {
	// Schema
	CreateSchema(ctx context.Context) error

	// Sessions
	CreateSession(ctx context.Context, rules Rules) (*Session, error)
	GetSession(ctx context.Context, sessionID string) (*Session, error)

	// Messages
	AddMessage(ctx context.Context, sessionID string, role string, content string, usage *Usage) (*Message, error)
	ListMessages(ctx context.Context, sessionID string) ([]Message, error)

	// Request Logs
	AddRequestLog(ctx context.Context, log RequestLog) (*RequestLog, error)
	UpdateRequestLog(ctx context.Context, id string, response string, status string, failReason string, errorMsg string, retryCount int, usage *Usage) error
}
