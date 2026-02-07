# ai

A Go package for **wrapping AI providers** with multi-turn conversation storage. Send prompts with structured rules, get JSON responses, and every conversation turn is logged to PostgreSQL with token usage metadata.

## Why I Built This

I kept rebuilding the same AI integration: call an API, parse the response, store the conversation. This package extracts that into a reusable library so any project can plug in AI with conversation history out of the box.

The key idea: **Provider** handles the AI call (stateless), **Store** handles persistence (sessions + messages). They don't know about each other — your app wires them together.

## Features

- **Multi-turn conversations** — sessions with ordered message history
- **Token usage tracking** — prompt, response, total, and thought tokens stored per message
- **Structured rules** — system prompt, output schema, and max tokens per session
- **Interface-based** — `Provider` for AI backends, `Store` for database
- **Gemini provider** — raw HTTP implementation (no SDK dependency)
- **PostgreSQL backend** — production-ready using pgx
- **Sequence ordering** — messages ordered by `seq` (not timestamps), guaranteed correct

## Install

```bash
go get github.com/meikuraledutech/ai
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/meikuraledutech/ai"
    "github.com/meikuraledutech/ai/gemini"
    "github.com/meikuraledutech/ai/postgres"
)

func main() {
    ctx := context.Background()

    pool, _ := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
    defer pool.Close()

    store := postgres.New(pool)
    provider := gemini.New(os.Getenv("GEMINI_API"), os.Getenv("MODEL_ID"))

    // Create tables
    store.CreateSchema(ctx)

    // Start a session with rules
    session, _ := store.CreateSession(ctx, ai.Rules{
        SystemPrompt: "You are a form builder. Return JSON with nodes and edges.",
        MaxTokens:    4096,
    })

    // First turn
    prompt := "Create a registration form with name, email, phone"
    history, _ := store.ListMessages(ctx, session.ID)
    result, _ := provider.Send(ctx, session.Rules, history, prompt)

    // Store both sides (usage only on assistant messages)
    store.AddMessage(ctx, session.ID, "user", prompt, nil)
    store.AddMessage(ctx, session.ID, "assistant", result.Content, &result.Usage)

    fmt.Printf("Response: %s\n", result.Content)
    fmt.Printf("Tokens: prompt=%d response=%d total=%d thought=%d\n",
        result.Usage.PromptTokens, result.Usage.ResponseTokens,
        result.Usage.TotalTokens, result.Usage.ThoughtTokens)

    // Second turn — history auto-builds
    prompt2 := "Add an address field after phone"
    history, _ = store.ListMessages(ctx, session.ID)
    result2, _ := provider.Send(ctx, session.Rules, history, prompt2)

    store.AddMessage(ctx, session.ID, "user", prompt2, nil)
    store.AddMessage(ctx, session.ID, "assistant", result2.Content, &result2.Usage)
}
```

## How It's Structured

```
ai/
├── ai.go           # Types: Rules, Usage, Message, Session, Result, Config
├── provider.go     # Provider interface + error definitions
├── store.go        # Store interface + error definitions
├── postgres/       # PostgreSQL implementation of Store
│   ├── postgres.go # PGStore struct, constructor
│   ├── schema.go   # Create/drop tables
│   ├── session.go  # Session CRUD
│   └── message.go  # Message logging with auto-seq
├── gemini/         # Gemini implementation of Provider
│   └── gemini.go   # Raw HTTP to Gemini REST API
└── example/        # Working demo
    └── main.go
```

The root package defines **types and interfaces** with zero dependencies. `postgres/` and `gemini/` are implementations you can swap.

## Conversation Flow

```
1. Create a session with rules (system prompt + output schema + max tokens)
2. Fetch message history (empty on first turn)
3. Send prompt + history to provider → get Result (content + usage)
4. Store user message (no usage)
5. Store assistant message (with usage)
6. Repeat from step 2 for next turn
```

Each message gets an auto-incremented `seq` number, so ordering is always correct regardless of timestamps.

## Token Usage Tracking

Every assistant message stores token counts from the provider:

```
session_id | seq | role      | content         | prompt | response | total | thought
-----------|-----|-----------|-----------------|--------|----------|-------|--------
sess_abc   |  1  | user      | "Create a..."   |    0   |     0    |    0  |    0
sess_abc   |  2  | assistant | '{"nodes":..}'  |   32   |   480    | 1501  |  989
sess_abc   |  3  | user      | "Add phone..."  |    0   |     0    |    0  |    0
sess_abc   |  4  | assistant | '{"nodes":..}'  |  523   |   576    | 1573  |  474
```

Find expensive sessions:

```sql
SELECT session_id, SUM(total_tokens) as total
FROM ai_messages
GROUP BY session_id
ORDER BY total DESC;
```

## Interfaces

### Provider

AI provider adapter. Implement this to add a new AI backend.

```go
type Provider interface {
    Send(ctx context.Context, rules Rules, history []Message, prompt string) (*Result, error)
}
```

### Store

Database persistence. Implement this to use a different database.

```go
type Store interface {
    CreateSchema(ctx context.Context) error
    DropSchema(ctx context.Context) error

    CreateSession(ctx context.Context, rules Rules) (*Session, error)
    GetSession(ctx context.Context, sessionID string) (*Session, error)

    AddMessage(ctx context.Context, sessionID string, role string, content string, usage *Usage) (*Message, error)
    ListMessages(ctx context.Context, sessionID string) ([]Message, error)
}
```

## Error Handling

| Error | Meaning |
|-------|---------|
| `ai.ErrEmptyPrompt` | Prompt string is empty |
| `ai.ErrProviderFailed` | AI provider returned an error |
| `ai.ErrSessionNotFound` | No session with the given ID |

```go
if errors.Is(err, ai.ErrProviderFailed) {
    // handle provider error
}
```

## Configuration

| Environment Variable | Required | Description |
|---------------------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `GEMINI_API` | Yes | Gemini API key |
| `MODEL_ID` | Yes | Gemini model ID (e.g., `gemini-3-flash-preview`) |

## Database Schema

Two tables:

- `ai_sessions` — session rules (system prompt, output schema, max tokens)
- `ai_messages` — conversation messages with seq ordering and token usage

Tables are created with `store.CreateSchema(ctx)` using `IF NOT EXISTS` — safe to call on every startup.

## Requirements

- Go 1.25+
- PostgreSQL (tested with Neon)
- `github.com/jackc/pgx/v5` (PostgreSQL driver)
- `github.com/google/uuid` (ID generation)

## Documentation

See [DOCS.md](DOCS.md) for the complete API reference with every type, method, and field documented.

## License

MIT License - see [LICENSE](LICENSE) for details.
