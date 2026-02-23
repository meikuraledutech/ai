package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/meikuraledutech/ai/v1"
)

const baseURL = "https://generativelanguage.googleapis.com/v1beta/models"
const maxAttempts = 2

// GeminiProvider implements ai.Provider using the Gemini REST API.
type GeminiProvider struct {
	apiKey  string
	modelID string
	client  *http.Client
	store   ai.Store
}

// New creates a new GeminiProvider.
func New(apiKey, modelID string) *GeminiProvider {
	return &GeminiProvider{
		apiKey:  apiKey,
		modelID: modelID,
		client:  &http.Client{},
		store:   nil,
	}
}

// WithStore configures request logging for this provider.
func (g *GeminiProvider) WithStore(store ai.Store) *GeminiProvider {
	g.store = store
	return g
}

// Send calls the Gemini generateContent API with validation and auto-retry.
// Validates JSON response by checking bracket matching. Auto-retries up to 2 times if validation fails.
func (g *GeminiProvider) Send(ctx context.Context, rules ai.Rules, history []ai.Message, prompt string) (*ai.Result, error) {
	if prompt == "" {
		return nil, ai.ErrEmptyPrompt
	}

	// Extract sessionID from history or context for logging
	sessionID := ""
	if len(history) > 0 {
		sessionID = history[0].SessionID
	}
	// Fallback: check context if history is empty (first message in session)
	if sessionID == "" {
		if ctxSessionID, ok := ctx.Value("session_id").(string); ok {
			sessionID = ctxSessionID
		}
	}

	// Initialize request log if store is available
	var logID string
	if g.store != nil {
		log, err := g.store.AddRequestLog(ctx, ai.RequestLog{
			SessionID:     sessionID,
			Prompt:        prompt,
			AttemptNumber: 1,
			FinalStatus:   ai.StatusPending,
		})
		if err == nil {
			logID = log.ID
		}
	}

	// Retry loop: up to 2 attempts
	var lastErr error
	var lastResult *ai.Result

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Send request to API
		result, err := g.sendOnce(ctx, rules, history, prompt)

		// Handle API errors
		if err != nil {
			failReason := classifyError(err)
			lastErr = err

			if g.store != nil && logID != "" {
				g.store.UpdateRequestLog(ctx, logID,
					"",                          // response
					ai.StatusFailed,             // status
					failReason,                  // fail_reason
					err.Error(),                 // error_message
					attempt-1,                   // retry_count
					nil,                         // usage
				)
			}

			// Retry if not last attempt
			if attempt < maxAttempts {
				continue
			}
			return nil, lastErr
		}

		// Validate JSON response
		valid, failReason := validateJSON(result.Content)
		if valid {
			// Success: JSON is valid
			if g.store != nil && logID != "" {
				g.store.UpdateRequestLog(ctx, logID,
					result.Content,           // response
					ai.StatusSuccess,         // status
					"",                       // fail_reason
					"",                       // error_message
					attempt-1,                // retry_count
					&result.Usage,            // usage
				)
			}
			return result, nil
		}

		// JSON validation failed
		lastResult = result

		if g.store != nil && logID != "" {
			g.store.UpdateRequestLog(ctx, logID,
				result.Content,           // response
				ai.StatusPending,         // status
				failReason,               // fail_reason
				"JSON validation failed", // error_message
				attempt-1,                // retry_count
				&result.Usage,            // usage
			)
		}

		// Retry if not last attempt
		if attempt < maxAttempts {
			// Add incomplete response and retry message to history for next attempt
			history = append(history,
				ai.Message{Role: "assistant", Content: result.Content},
				ai.Message{Role: "user", Content: "Your previous response had incomplete JSON (mismatched brackets). Please regenerate the complete, valid JSON response."},
			)
			continue
		}

		// Max attempts exceeded
		if g.store != nil && logID != "" {
			g.store.UpdateRequestLog(ctx, logID,
				lastResult.Content,               // response
				ai.StatusFailed,                  // status
				ai.FailReasonMaxRetries,          // fail_reason
				"JSON validation failed after max retries", // error_message
				attempt-1,                        // retry_count
				&lastResult.Usage,                // usage
			)
		}

		return nil, fmt.Errorf("ai: JSON validation failed after %d attempts: %w", maxAttempts, ai.ErrProviderFailed)
	}

	return nil, lastErr
}

// sendOnce makes a single API request without validation or retry.
func (g *GeminiProvider) sendOnce(ctx context.Context, rules ai.Rules, history []ai.Message, prompt string) (*ai.Result, error) {
	reqBody := g.buildRequest(rules, history, prompt)

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ai: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", baseURL, g.modelID, g.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("ai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai: send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d: %s", ai.ErrProviderFailed, resp.StatusCode, string(body))
	}

	return g.parseResponse(body)
}

func (g *GeminiProvider) buildRequest(rules ai.Rules, history []ai.Message, prompt string) map[string]any {
	contents := make([]map[string]any, 0, len(history)+1)

	for _, msg := range history {
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]any{{"text": msg.Content}},
		})
	}

	contents = append(contents, map[string]any{
		"role":  "user",
		"parts": []map[string]any{{"text": prompt}},
	})

	req := map[string]any{
		"contents": contents,
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
		},
	}

	if rules.MaxTokens > 0 {
		req["generationConfig"].(map[string]any)["maxOutputTokens"] = rules.MaxTokens
	}

	if rules.OutputSchema != "" {
		var schema map[string]any
		if err := json.Unmarshal([]byte(rules.OutputSchema), &schema); err == nil {
			req["generationConfig"].(map[string]any)["responseSchema"] = schema
		}
	}

	if rules.SystemPrompt != "" {
		req["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": rules.SystemPrompt}},
		}
	}

	return req
}

func (g *GeminiProvider) parseResponse(body []byte) (*ai.Result, error) {
	var resp geminiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ai: parse response: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("%w: empty response from Gemini", ai.ErrProviderFailed)
	}

	return &ai.Result{
		Content: resp.Candidates[0].Content.Parts[0].Text,
		Usage: ai.Usage{
			PromptTokens:   resp.UsageMetadata.PromptTokenCount,
			ResponseTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:    resp.UsageMetadata.TotalTokenCount,
			ThoughtTokens:  resp.UsageMetadata.ThoughtsTokenCount,
		},
	}, nil
}

// validateJSON checks if JSON is complete by counting brackets.
// Returns (valid, failReason) - if invalid, failReason indicates the type of error.
func validateJSON(s string) (bool, string) {
	curly, square := 0, 0
	for _, c := range s {
		switch c {
		case '{':
			curly++
		case '}':
			curly--
		case '[':
			square++
		case ']':
			square--
		}
	}
	if curly != 0 || square != 0 {
		return false, ai.FailReasonIncompleteJSON
	}
	return true, ""
}

// classifyError categorizes an error to determine the fail reason.
func classifyError(err error) string {
	if err == context.DeadlineExceeded {
		return ai.FailReasonTimeout
	}

	// Check for net errors (network/timeout)
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return ai.FailReasonTimeout
		}
		return ai.FailReasonNetworkError
	}

	// Check for context errors
	if err == context.Canceled {
		return ai.FailReasonNetworkError
	}

	// Default to unknown error
	return ai.FailReasonUnknownError
}

// Gemini API response types.
type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata geminiUsage       `json:"usageMetadata"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
	ThoughtsTokenCount   int `json:"thoughtsTokenCount"`
}

// Ensure GeminiProvider implements ai.Provider at compile time.
var _ ai.Provider = (*GeminiProvider)(nil)
