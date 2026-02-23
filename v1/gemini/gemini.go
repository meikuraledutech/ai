package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/meikuraledutech/ai/v1"
)

const baseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// GeminiProvider implements ai.Provider using the Gemini REST API.
type GeminiProvider struct {
	apiKey  string
	modelID string
	client  *http.Client
}

// New creates a new GeminiProvider.
func New(apiKey, modelID string) *GeminiProvider {
	return &GeminiProvider{
		apiKey:  apiKey,
		modelID: modelID,
		client:  &http.Client{},
	}
}

// Send calls the Gemini generateContent API and returns the response + usage.
func (g *GeminiProvider) Send(ctx context.Context, rules ai.Rules, history []ai.Message, prompt string) (*ai.Result, error) {
	if prompt == "" {
		return nil, ai.ErrEmptyPrompt
	}

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
