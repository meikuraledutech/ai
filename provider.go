package ai

import (
	"context"
	"errors"
)

var (
	ErrEmptyPrompt    = errors.New("ai: prompt is empty")
	ErrProviderFailed = errors.New("ai: provider error")
)

// Provider defines the contract for AI providers.
type Provider interface {
	Send(ctx context.Context, rules Rules, history []Message, prompt string) (*Result, error)
}
