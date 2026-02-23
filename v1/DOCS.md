# AI Package — Complete Reference

Interface-based AI provider wrapper with multi-turn conversation storage. Code against `ai.Provider` and `ai.Store`, swap backends freely.

---

## Table of Contents

1. [Installation & Setup](#installation--setup)
2. [Database Schema](#database-schema)
3. [Types](#types)
4. [Sentinel Errors](#sentinel-errors)
5. [Provider Interface](#provider-interface)
6. [Store Interface](#store-interface)
7. [Schema Operations](#schema-operations)
8. [Session Operations](#session-operations)
   - [CreateSession](#createsession)
   - [GetSession](#getsession)
9. [Message Operations](#message-operations)
   - [AddMessage](#addmessage)
   - [ListMessages](#listmessages)
10. [Request Logging (Validator)](#request-logging-validator)
11. [Gemini Provider](#gemini-provider)
12. [Error Handling Guide](#error-handling-guide)
13. [Token Usage Queries](#token-usage-queries)
14. [Migration & Schema Management](#migration--schema-management)

---

## Installation & Setup

```bash
go get github.com/meikuraledutech/ai
```

```go
import (
    "github.com/meikuraledutech/ai"
    "github.com/meikuraledutech/ai/postgres"
    "github.com/meikuraledutech/ai/gemini"
)
```

### Initialize

```go
pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
if err != nil {
    log.Fatal(err)
}

var store ai.Store = postgres.New(pool)
var provider ai.Provider = gemini.New(os.Getenv("GEMINI_API"), os.Getenv("MODEL_ID"))

// Create tables on startup (idempotent, safe to call every time)
if err := store.CreateSchema(ctx); err != nil {
    log.Fatal(err)
}
```

### Folder Structure

```
ai/
├── ai.go           # Types: Rules, Usage, Message, Session, Result, Config
├── provider.go     # Provider interface + sentinel errors
├── store.go        # Store interface + sentinel errors
├── postgres/
│   ├── postgres.go # PGStore struct, New()
│   ├── schema.go   # CreateSchema, DropSchema
│   ├── session.go  # CreateSession, GetSession
│   └── message.go  # AddMessage, ListMessages
├── gemini/
│   └── gemini.go   # Gemini REST API implementation
└── example/
    └── main.go     # Working demo
```

---

## Database Schema

Three tables. Sessions hold rules, messages hold the conversation, request_logs track API usage.

```sql
CREATE TABLE IF NOT EXISTS ai_sessions (
    id            TEXT PRIMARY KEY,
    system_prompt TEXT NOT NULL DEFAULT '',
    output_schema TEXT NOT NULL DEFAULT '',
    max_tokens    INT NOT NULL DEFAULT 4096,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_messages (
    id              TEXT PRIMARY KEY,
    session_id      TEXT NOT NULL REFERENCES ai_sessions(id) ON DELETE CASCADE,
    seq             INT NOT NULL,
    role            TEXT NOT NULL,
    content         TEXT NOT NULL,
    prompt_tokens   INT NOT NULL DEFAULT 0,
    response_tokens INT NOT NULL DEFAULT 0,
    total_tokens    INT NOT NULL DEFAULT 0,
    thought_tokens  INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(session_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_ai_messages_session ON ai_messages(session_id);
```

**Key points:**
- `ON DELETE CASCADE` — deleting a session auto-deletes all its messages
- `UNIQUE(session_id, seq)` — guarantees message ordering per session
- `seq` is auto-incremented by `AddMessage` (not a DB sequence — computed via `MAX(seq) + 1`)
- Token columns default to 0 — user messages have no usage, assistant messages carry the counts

---

## Types

### Rules

Controls AI behavior for a session. Stored in the `ai_sessions` table.

```go
type Rules struct {
    SystemPrompt string `json:"system_prompt"` // system instruction for the AI
    OutputSchema string `json:"output_schema"` // JSON schema string for structured output
    MaxTokens    int    `json:"max_tokens"`    // max output tokens
}
```

| Field | Type | Description |
|-------|------|-------------|
| `system_prompt` | `string` | System instruction prepended to every request. Tells the AI how to behave. |
| `output_schema` | `string` | JSON schema string. If set, Gemini uses `responseSchema` for structured output. |
| `max_tokens` | `int` | Maximum output tokens. Maps to `maxOutputTokens` in Gemini. |

### Usage

Token counts from the AI provider response. Stored per assistant message.

```go
type Usage struct {
    PromptTokens   int `json:"prompt_tokens"`   // tokens in the input
    ResponseTokens int `json:"response_tokens"` // tokens in the output
    TotalTokens    int `json:"total_tokens"`    // prompt + response + thought
    ThoughtTokens  int `json:"thought_tokens"`  // internal reasoning tokens
}
```

| Field | Gemini mapping |
|-------|---------------|
| `prompt_tokens` | `usageMetadata.promptTokenCount` |
| `response_tokens` | `usageMetadata.candidatesTokenCount` |
| `total_tokens` | `usageMetadata.totalTokenCount` |
| `thought_tokens` | `usageMetadata.thoughtsTokenCount` |

### Message

A single turn in a conversation. Ordered by `seq` within a session.

```go
type Message struct {
    ID        string    `json:"id"`              // UUID, auto-generated
    SessionID string    `json:"session_id"`      // parent session
    Seq       int       `json:"seq"`             // 1, 2, 3... guaranteed order
    Role      string    `json:"role"`            // "user" or "assistant"
    Content   string    `json:"content"`         // message text or JSON
    Usage     *Usage    `json:"usage,omitempty"` // token usage (assistant only)
    CreatedAt time.Time `json:"created_at"`      // set by database
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | UUID, auto-generated by `AddMessage` |
| `session_id` | `string` | FK to `ai_sessions.id` |
| `seq` | `int` | Auto-incremented per session. 1-indexed. |
| `role` | `string` | `"user"` for prompts, `"assistant"` for AI responses |
| `content` | `string` | The message text. For assistant messages, typically JSON. |
| `usage` | `*Usage` | Token counts. `nil` for user messages, populated for assistant messages. |
| `created_at` | `time.Time` | Set by PostgreSQL `NOW()` |

### Session

Groups messages into a conversation with shared rules.

```go
type Session struct {
    ID        string    `json:"id"`         // UUID, auto-generated
    Rules     Rules     `json:"rules"`      // AI behavior config
    CreatedAt time.Time `json:"created_at"` // set by database
}
```

### Result

What the provider returns — content plus token usage.

```go
type Result struct {
    Content string `json:"content"` // the AI response text/JSON
    Usage   Usage  `json:"usage"`   // token counts for this turn
}
```

### Config

Package-level defaults.

```go
type Config struct {
    DefaultMaxTokens int // default: 4096
}
```

Use `ai.DefaultConfig()` for sensible defaults.

### RequestLog

Tracks every AI request attempt for cost analysis and debugging.

```go
type RequestLog struct {
    ID            string    // unique request log ID
    SessionID     string    // which session this request belongs to
    Prompt        string    // the user prompt sent
    Response      string    // the AI response (or partial if truncated)
    AttemptNumber int       // which attempt (1 or 2 with auto-retry)
    RetryCount    int       // how many retries were attempted
    FinalStatus   string    // "success" or "failed"
    FailReason    string    // why it failed: incomplete_json, network_error, timeout, api_error, etc
    ErrorMessage  string    // detailed error description if failed
    Usage         Usage     // token counts
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

### Fail Reason Constants

```go
ai.FailReasonIncompleteJSON // JSON brackets don't match, response truncated
ai.FailReasonInvalidJSON    // JSON malformed
ai.FailReasonNetworkError   // Connection/network failure
ai.FailReasonTimeout        // Request exceeded time limit
ai.FailReasonAPIError       // AI API returned error
ai.FailReasonMaxRetries     // Failed after max retries
ai.FailReasonUnknownError   // Other unexpected errors
```

### Status Constants

```go
ai.StatusSuccess // Request succeeded
ai.StatusFailed  // Request failed
ai.StatusPending // Request pending
```

---

## Sentinel Errors

```go
ai.ErrEmptyPrompt    // "ai: prompt is empty"
ai.ErrProviderFailed // "ai: provider error"
ai.ErrSessionNotFound // "ai: session not found"
```

Check with `errors.Is()`:

```go
if errors.Is(err, ai.ErrEmptyPrompt) { ... }
if errors.Is(err, ai.ErrProviderFailed) { ... }
if errors.Is(err, ai.ErrSessionNotFound) { ... }
```

---

## Provider Interface

```go
type Provider interface {
    Send(ctx context.Context, rules Rules, history []Message, prompt string) (*Result, error)
}
```

### Send

Sends a prompt to the AI provider with conversation history and returns the response.

**Parameters:**
- `rules` — AI behavior config (system prompt, output schema, max tokens)
- `history` — previous messages in this session (pass result of `ListMessages`)
- `prompt` — the new user message

**Returns:**
- `*Result` — content string + token usage
- `error` — `ErrEmptyPrompt` if prompt is empty, `ErrProviderFailed` if the API call fails

**Behavior:**
- Provider is **stateless** — no DB, no session awareness
- History is passed in so the provider can build multi-turn context
- Role mapping: `"user"` stays `"user"`, `"assistant"` is mapped to the provider's equivalent (e.g., `"model"` for Gemini)
- The `prompt` parameter is appended as the final user message (not included in history)

---

## Store Interface

```go
type Store interface {
    CreateSchema(ctx context.Context) error

    CreateSession(ctx context.Context, rules Rules) (*Session, error)
    GetSession(ctx context.Context, sessionID string) (*Session, error)

    AddMessage(ctx context.Context, sessionID string, role string, content string, usage *Usage) (*Message, error)
    ListMessages(ctx context.Context, sessionID string) ([]Message, error)

    AddRequestLog(ctx context.Context, log RequestLog) (*RequestLog, error)
    UpdateRequestLog(ctx context.Context, id string, response string, status string, failReason string, errorMsg string, retryCount int, usage *Usage) error
}
```

---

## Schema Operations

### CreateSchema

```
CreateSchema(ctx context.Context) error
```

Creates `ai_sessions` and `ai_messages` tables with indexes. **Idempotent** — uses `IF NOT EXISTS`, safe to call on every app startup.

| Scenario | Returns |
|----------|---------|
| Tables created or already exist | `nil` |
| DB connection failed | `error` (wrapped DB error) |

### DropSchema

```
DropSchema(ctx context.Context) error
```

Drops both tables with `CASCADE`. **Idempotent** — uses `IF EXISTS`. **Destructive — all data is lost.**

| Scenario | Returns |
|----------|---------|
| Tables dropped or didn't exist | `nil` |
| DB connection failed | `error` (wrapped DB error) |

---

## Session Operations

### CreateSession

```
CreateSession(ctx context.Context, rules Rules) (*Session, error)
```

Creates a new conversation session with the given rules. Generates a UUID for the session ID.

| Scenario | Returns |
|----------|---------|
| Success | `*Session` with ID, Rules, CreatedAt |
| DB error | `nil, error` (prefixed `"ai: create session:"`) |

**Go usage:**

```go
session, err := store.CreateSession(ctx, ai.Rules{
    SystemPrompt: "You are a form builder. Return JSON with nodes and edges.",
    OutputSchema: `{"type":"object","properties":{"nodes":{"type":"array"},"edges":{"type":"array"}}}`,
    MaxTokens:    4096,
})
// session.ID → "43394978-fc19-42ee-920d-6582048679ab"
```

### GetSession

```
GetSession(ctx context.Context, sessionID string) (*Session, error)
```

Retrieves a session by ID, including its rules.

| Scenario | Returns |
|----------|---------|
| Found | `*Session` with ID, Rules, CreatedAt |
| Not found | `nil, error` (wrapped pgx `no rows` error) |
| DB error | `nil, error` (prefixed `"ai: get session:"`) |

**Go usage:**

```go
session, err := store.GetSession(ctx, "43394978-fc19-42ee-920d-6582048679ab")
if err != nil {
    // session not found or DB error
}
// session.Rules.SystemPrompt → "You are a form builder..."
```

---

## Message Operations

### AddMessage

```
AddMessage(ctx context.Context, sessionID string, role string, content string, usage *Usage) (*Message, error)
```

Appends a message to a session. The `seq` is auto-incremented (computed as `MAX(seq) + 1` for the session).

**Parameters:**
- `sessionID` — the session to add the message to
- `role` — `"user"` or `"assistant"`
- `content` — the message text or JSON string
- `usage` — token counts (pass `nil` for user messages, `&result.Usage` for assistant messages)

| Scenario | Returns |
|----------|---------|
| Success | `*Message` with ID, Seq, CreatedAt |
| Session doesn't exist | `nil, error` (FK violation) |
| DB error | `nil, error` (prefixed `"ai: add message:"`) |

**Go usage:**

```go
// Store user message (no usage)
msg, err := store.AddMessage(ctx, session.ID, "user", "Create a registration form", nil)
// msg.Seq → 1

// Store assistant message (with usage)
msg2, err := store.AddMessage(ctx, session.ID, "assistant", result.Content, &result.Usage)
// msg2.Seq → 2
// msg2.Usage.TotalTokens → 1501
```

### ListMessages

```
ListMessages(ctx context.Context, sessionID string) ([]Message, error)
```

Returns all messages for a session, ordered by `seq ASC`. Returns `nil` if no messages found.

| Scenario | Returns |
|----------|---------|
| Messages found | `[]Message{...}` ordered by seq |
| No messages | `nil, nil` |
| DB error | `nil, error` (prefixed `"ai: list messages:"`) |

**Go usage:**

```go
messages, err := store.ListMessages(ctx, session.ID)
// messages[0].Seq → 1, messages[0].Role → "user"
// messages[1].Seq → 2, messages[1].Role → "assistant", messages[1].Usage.TotalTokens → 1501
```

**Important:** Pass the result of `ListMessages` as `history` to `Provider.Send()`:

```go
history, _ := store.ListMessages(ctx, session.ID)
result, err := provider.Send(ctx, session.Rules, history, "next prompt")
```

---

## Request Logging (Validator)

Automatically logs every API request with validation, auto-retry on failure, and detailed error tracking. Useful for cost analysis, debugging, and compliance.

### Enable Request Logging

```go
provider := gemini.New(apiKey, modelID).WithStore(store)
// Now all Send() calls are logged to ai_request_logs table
```

### What Gets Logged

Each request creates a log entry with:
- **Session ID** — which conversation session
- **Prompt & Response** — the full request and response content
- **Attempt Number & Retry Count** — how many times it was retried
- **Final Status** — `"success"` or `"failed"`
- **Fail Reason** — why it failed (if failed)
- **Token Counts** — prompt, response, total, and thought tokens
- **Timestamps** — when created and last updated

### Automatic Validation & Retry

The provider automatically:
1. **Validates JSON completeness** — checks that `{` and `[` counts match `}` and `]`
2. **Retries on validation failure** — up to 2 attempts total
3. **Logs each attempt** — including partial responses
4. **Classifies errors** — incomplete_json, network_error, timeout, api_error, etc

Example error flow:
- Attempt 1: API returns truncated JSON → validation fails → logged
- Attempt 2: Same prompt sent again → complete JSON → success → logged
- Result: 2 log entries, retry_count=1, final status=success

### Query Request Logs

```go
// Get all requests for a session
rows, err := db.Query(ctx, `
  SELECT id, final_status, fail_reason, retry_count, total_tokens, created_at
  FROM ai_request_logs
  WHERE session_id = $1
  ORDER BY created_at DESC
`, sessionID)

// Cost analysis: count successes vs failures
rows, err := db.Query(ctx, `
  SELECT final_status, COUNT(*) as count, SUM(total_tokens) as tokens
  FROM ai_request_logs
  WHERE created_at > NOW() - INTERVAL '1 day'
  GROUP BY final_status
`)

// Find problematic requests
rows, err := db.Query(ctx, `
  SELECT id, fail_reason, error_message
  FROM ai_request_logs
  WHERE final_status = 'failed'
  ORDER BY created_at DESC
  LIMIT 10
`)
```

### Error Classification

| Fail Reason | Meaning | Recoverable |
|-------------|---------|-------------|
| `incomplete_json` | Truncated due to token limit | ✓ (retried) |
| `invalid_json` | Malformed response | ✗ |
| `network_error` | Connection failed | ✓ (could retry) |
| `timeout` | Request exceeded deadline | ✓ (could retry) |
| `api_error` | AI provider returned error | ✗ |
| `max_retries_exceeded` | Failed after 2 attempts | ✗ |
| `unknown_error` | Unexpected error | ? |

---

## Gemini Provider

Raw HTTP implementation using the Gemini REST API. No SDK dependency.

### Constructor

```go
import "github.com/meikuraledutech/ai/gemini"

provider := gemini.New(apiKey, modelID)
// apiKey  → your Gemini API key
// modelID → e.g., "gemini-3-flash-preview"
```

### API Endpoint

```
POST https://generativelanguage.googleapis.com/v1beta/models/{modelID}:generateContent?key={apiKey}
```

### Request Mapping

| ai package | Gemini API |
|------------|-----------|
| `rules.SystemPrompt` | `systemInstruction.parts[0].text` |
| `rules.OutputSchema` | `generationConfig.responseSchema` (parsed from JSON string) |
| `rules.MaxTokens` | `generationConfig.maxOutputTokens` |
| `history[].Role == "user"` | `contents[].role = "user"` |
| `history[].Role == "assistant"` | `contents[].role = "model"` |
| `prompt` | Final `contents[]` entry with `role: "user"` |
| — | `generationConfig.responseMimeType = "application/json"` (always set) |

### Response Mapping

| Gemini API | ai package |
|-----------|-----------|
| `candidates[0].content.parts[0].text` | `Result.Content` |
| `usageMetadata.promptTokenCount` | `Result.Usage.PromptTokens` |
| `usageMetadata.candidatesTokenCount` | `Result.Usage.ResponseTokens` |
| `usageMetadata.totalTokenCount` | `Result.Usage.TotalTokens` |
| `usageMetadata.thoughtsTokenCount` | `Result.Usage.ThoughtTokens` |

### Error Handling

| Scenario | Error |
|----------|-------|
| Empty prompt | `ai.ErrEmptyPrompt` |
| HTTP status != 200 | `ai.ErrProviderFailed` (wrapped with status code and body) |
| Empty response (no candidates) | `ai.ErrProviderFailed` (wrapped with "empty response") |
| JSON parse error | Wrapped `encoding/json` error |

---

## Error Handling Guide

### All possible error types

| Error | Type | When | How to check |
|-------|------|------|-------------|
| Empty prompt | Sentinel | `Provider.Send` with empty string | `errors.Is(err, ai.ErrEmptyPrompt)` |
| Provider failed | Sentinel | API returned non-200 or empty response | `errors.Is(err, ai.ErrProviderFailed)` |
| Session not found | Sentinel | `GetSession` with unknown ID | `errors.Is(err, ai.ErrSessionNotFound)` |
| FK violation | DB | `AddMessage` to non-existent session | Wrapped pgx error |
| Connection error | DB | Any method | Wrapped pgx error |

### Error prefix convention

All errors from the postgres implementation are prefixed with `"ai: "`:

```
"ai: prompt is empty"
"ai: provider error"
"ai: session not found"
"ai: create session: ..."
"ai: get session: ..."
"ai: add message: ..."
"ai: list messages: ..."
"ai: scan message: ..."
```

### Error handling pattern (Fiber)

```go
func handleError(c fiber.Ctx, err error) error {
    if err == nil {
        return nil
    }

    if errors.Is(err, ai.ErrEmptyPrompt) {
        return c.Status(400).JSON(fiber.Map{"error": "prompt is empty"})
    }
    if errors.Is(err, ai.ErrProviderFailed) {
        return c.Status(502).JSON(fiber.Map{"error": "ai provider failed"})
    }
    if errors.Is(err, ai.ErrSessionNotFound) {
        return c.Status(404).JSON(fiber.Map{"error": "session not found"})
    }

    return c.Status(500).JSON(fiber.Map{"error": err.Error()})
}
```

---

## Token Usage Queries

### Total tokens per session

```sql
SELECT session_id, SUM(total_tokens) as total
FROM ai_messages
WHERE role = 'assistant'
GROUP BY session_id
ORDER BY total DESC;
```

### Most expensive single turn

```sql
SELECT id, session_id, seq, total_tokens, thought_tokens
FROM ai_messages
WHERE role = 'assistant'
ORDER BY total_tokens DESC
LIMIT 10;
```

### Sessions with more than N turns

```sql
SELECT session_id, COUNT(*) as turns
FROM ai_messages
GROUP BY session_id
HAVING COUNT(*) > 10
ORDER BY turns DESC;
```

### Average tokens per session

```sql
SELECT
    session_id,
    COUNT(*) FILTER (WHERE role = 'assistant') as ai_turns,
    SUM(total_tokens) as total_tokens,
    ROUND(AVG(total_tokens) FILTER (WHERE role = 'assistant')) as avg_per_turn
FROM ai_messages
GROUP BY session_id
ORDER BY total_tokens DESC;
```

### Daily token consumption

```sql
SELECT
    DATE(created_at) as day,
    SUM(total_tokens) as tokens,
    COUNT(DISTINCT session_id) as sessions
FROM ai_messages
WHERE role = 'assistant'
GROUP BY DATE(created_at)
ORDER BY day DESC;
```

---

## Migration & Schema Management

### First-time setup

Call `CreateSchema` on app startup. Uses `IF NOT EXISTS` — safe to run every time.

```go
store.CreateSchema(ctx)
```

### Drop everything (destructive)

```go
store.DropSchema(ctx)
```

### Reset (drop + recreate)

```go
store.DropSchema(ctx)
store.CreateSchema(ctx)
```

### Adding columns (future migrations)

```sql
-- Example: add a "model" column to track which model was used per message
ALTER TABLE ai_messages ADD COLUMN IF NOT EXISTS model TEXT DEFAULT '';

-- Example: add session metadata
ALTER TABLE ai_sessions ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';
```

### Checking current schema

```bash
psql $DATABASE_URL -c "\d ai_sessions"
psql $DATABASE_URL -c "\d ai_messages"
```

---

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `GEMINI_API` | Yes | Gemini API key |
| `MODEL_ID` | Yes | Gemini model ID (e.g., `gemini-3-flash-preview`) |
