# AI Package - Versioned

Go package for **wrapping AI providers with multi-turn conversation storage**. Send prompts with structured rules, get JSON responses, and every conversation turn is logged to PostgreSQL with token usage metadata.

## Versions

### V1 (Current)

```bash
go get github.com/meikuraledutech/ai@v1.0.1
```

**Import:**
```go
import (
    "github.com/meikuraledutech/ai/v1"
    "github.com/meikuraledutech/ai/v1/postgres"
    "github.com/meikuraledutech/ai/v1/gemini"
)

// Usage
store := postgres.New(pool)
provider := gemini.New(apiKey, modelID)
if err := store.CreateSchema(ctx); err != nil {
    log.Fatal(err)
}
```

**Documentation:** See [v1/README.md](v1/README.md) and [v1/DOCS.md](v1/DOCS.md)

### V2 (Coming)

Reserved for future versions with new interfaces and methods.

## Directory Structure

```
ai/
├── v1/                    ← Current stable version
│   ├── ai.go             ← Core types and interfaces
│   ├── provider.go       ← Provider interface
│   ├── store.go          ← Store interface
│   ├── postgres/         ← PostgreSQL implementation with migrations
│   ├── gemini/           ← Gemini provider implementation
│   ├── example/          ← Usage examples
│   ├── README.md         ← Full documentation
│   ├── DOCS.md           ← Complete API reference
│   └── LICENSE
├── v2/                    ← Next major version (empty)
└── go.mod               ← Root module

```

## Getting Started

### Installation

```bash
go get github.com/meikuraledutech/ai@v1.0.1
```

### Basic Usage

See [v1/README.md](v1/README.md) for complete quick start guide.

## Key Features

- **Multi-turn conversations** with ordered message history
- **Token usage tracking** (prompt, response, total, thought)
- **Structured rules** per session (system prompt, output schema, max tokens)
- **JSON validator with auto-retry** — validates bracket matching, retries up to 2 times on failure
- **Request logging** — logs every API request with session context, fail reasons, and token counts
- **Interface-based design** for extensibility
- **Gemini provider** (raw HTTP, no SDK)
- **PostgreSQL backend** with migration tracking
- **Sequence-based message ordering** (guaranteed correct order)
- **Cost analysis** — queryable request logs for usage and failure tracking

## Imports Changed

**Old (before v1):**
```go
import "github.com/meikuraledutech/ai"
```

**New (V1):**
```go
import "github.com/meikuraledutech/ai/v1"
```

## Documentation

- **[V1 Full README](v1/README.md)** — Quick start and overview
- **[V1 API Docs](v1/DOCS.md)** — Complete API reference with all types and methods
