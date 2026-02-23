# Changelog

All notable changes to the AI package are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-02-23

### Added

#### Migration System
- **Embedded SQL migrations** with automatic schema management
  - Migration files embedded in binary with `//go:embed` directive
  - Deterministic ordering using numeric prefixes (001, 002, etc.)
  - SHA256 checksums for integrity verification and modification detection
  - Transactional schema application with automatic rollback on error

- **Migration tracking** in PostgreSQL
  - `ai_migrations` table to track applied migrations
  - Tracks migration name, applied status, applied timestamp, and checksum
  - Prevents re-application of identical migrations
  - Detects modified migrations by comparing checksums

- **Migration operations**
  - `Migrate()` - Apply pending migrations in order
  - `Rollback()` - Revert last applied migration
  - `MigrationStatus()` - Query status of all migrations

- **Database schema** (001_initial_schema)
  - `ai_sessions` table - Group messages into conversations
    - `id` (UUID primary key)
    - `system_prompt` (text)
    - `output_schema` (text, optional)
    - `max_tokens` (integer)
    - `created_at` (timestamp)
  - `ai_messages` table - Store individual messages
    - `id` (UUID primary key)
    - `session_id` (FK to ai_sessions)
    - `seq` (message sequence number)
    - `role` (user/assistant)
    - `content` (message text)
    - Token fields: `prompt_tokens`, `response_tokens`, `total_tokens`, `thought_tokens`
    - `created_at` (timestamp)
  - Foreign key constraint with ON DELETE CASCADE
  - Unique constraint on (session_id, seq) for ordering guarantee
  - Index on session_id for query performance

#### Configuration Management
- **Environment-based configuration** via `LoadConfig()`
  - `DATABASE_URL` - PostgreSQL connection string
  - `GEMINI_API` - Gemini API key
  - `MODEL_ID` - AI model identifier
  - `MAX_TOKENS` - Token limit for responses (default: 16384)

- **Default values**
  - Default MAX_TOKENS: 16,384 (16k tokens)
  - Fallback to hardcoded defaults if environment variables not set
  - Runtime override capability without code changes

#### Token Limit Handling
- **Graceful token limit enforcement**
  - API respects `maxOutputTokens` configuration
  - Responses truncated gracefully when approaching limits
  - No error handling required - API returns HTTP 200
  - Token usage accurately tracked regardless of truncation

- **Token limit testing**
  - `large-prompt-test` example demonstrates behavior
  - Tests with comprehensive 605-character enterprise architecture request
  - Verified with 16k token limit: 6,586 tokens used (40.2%)
  - Documents API behavior for over-limit requests

- **Token management strategies**
  - Option A: Increase MAX_TOKENS via environment variable
  - Option B: Split large requests across multiple conversation turns
  - Option C: Request summarized/condensed responses
  - Token estimation approach documented

#### JSON Validator with Auto-Retry
- **JSON completion validation** via bracket matching
  - Validates that `{` and `[` counts match `}` and `]`
  - Detects truncated/incomplete JSON responses
  - Works with any JSON structure

- **Automatic retry logic**
  - Retries up to 2 times on validation failure
  - Appends retry context to conversation history
  - Auto-classifies failure reasons for debugging

- **Request logging with fail reasons**
  - `ai_request_logs` table tracks every API request attempt
  - Logs session_id, prompt, response, attempt_number, retry_count
  - Records final_status (success/failed) and fail_reason
  - Captures token usage for cost analysis
  - Fail reasons: incomplete_json, invalid_json, network_error, timeout, api_error, max_retries_exceeded, unknown_error

- **Optional cost tracking**
  - `WithStore()` method enables opt-in request logging on provider
  - Query logs for cost analysis and failure patterns
  - SessionID passed via context for first-message logging
  - Indexes on session_id and final_status for efficient querying

#### Version Structure
- **Go module versioning** with `/v1` suffix
  - Module path: `github.com/meikuraledutech/ai/v1`
  - Allows parallel major versions in future
  - Clean dependency management per version

- **Directory reorganization**
  - All code moved to `v1/` directory
  - Preserves auth/v1 pattern for consistency
  - `v2/` reserved for future major version

#### Documentation
- **README.md** - Root level versioning guide
  - Quick start instructions
  - Directory structure explanation
  - Upgrade path for future versions

- **v1/README.md** - Quick start guide
  - Installation instructions
  - Basic usage example
  - Configuration reference

- **v1/DOCS.md** - Comprehensive API documentation
  - Complete API reference
  - Type definitions
  - Function signatures and behavior
  - Error handling guidelines

- **v1/example/LARGE_PROMPT_TEST_RESULTS.md** - Token limit documentation
  - Test execution results
  - Token usage breakdown
  - API behavior analysis
  - Recommended approaches for large requests
  - Database persistence verification

- **MIGRATION_COMPLETED.md** - Implementation details
  - Complete migration checklist
  - All files moved to v1/
  - Pattern alignment with auth/v1
  - Testing and verification results

#### Examples
- **v1/example/main.go** - Basic multi-turn conversation
  - Environment-based configuration
  - Session creation with rules
  - Message storage and retrieval
  - Multi-turn conversation demonstration

- **v1/example/large-prompt-test/main.go** - Token limit testing
  - Large prompt handling
  - Token usage analysis
  - Completion status verification
  - Response preview display

### Changed

#### Type Updates
- **Rules struct** - Token limit now configurable per session
  - Added `MaxTokens` field to override default
  - Preserved `SystemPrompt` for per-session behavior
  - Preserved `OutputSchema` for structured output

- **Config type** - Renamed to distinguish application vs library config
  - Library `Config` with `DefaultMaxTokens` constant
  - New `AppConfig` for runtime environment-based configuration

#### API Changes
- **schema.go** - CreateSchema() now delegates to migration system
  - Automatic migration application
  - Idempotent schema creation
  - Migration tracking enabled

- **Message storage** - Token fields now properly tracked
  - `AddMessage()` accepts `*Usage` parameter
  - Token counts persisted with message
  - Supports all token types: prompt, response, total, thought

#### Import Changes
- All imports updated to use `/v1` module path
  - Example: `github.com/meikuraledutech/ai/v1`
  - Example: `github.com/meikuraledutech/ai/v1/postgres`
  - Example: `github.com/meikuraledutech/ai/v1/gemini`

### Removed

#### Legacy Root-Level Files
- Removed `/ai.go` - Moved to `v1/ai.go`
- Removed `/provider.go` - Moved to `v1/provider.go`
- Removed `/store.go` - Moved to `v1/store.go`
- Removed `/postgres/` directory - Moved to `v1/postgres/`
- Removed `/gemini/` directory - Moved to `v1/gemini/`
- Removed `/example/` directory - Moved to `v1/example/`

#### Hardcoded Configuration
- Removed hardcoded token limits from code
- Removed hardcoded database URLs from code
- Removed hardcoded model IDs from code
- All configuration now environment-based

### Fixed

- Token limit enforcement now properly delegated to provider API
- Database connection string no longer hardcoded
- Schema creation now idempotent via migration system
- Message ordering guaranteed via sequence number unique constraint

### Migration Guide

#### For existing users upgrading from unversioned code:

1. **Update import paths**
   ```go
   // Old
   import "github.com/meikuraledutech/ai/v1"
   // Now
   import "github.com/meikuraledutech/ai/v1"
   ```

2. **Environment configuration**
   ```bash
   # Set required environment variables
   DATABASE_URL=postgres://user:pass@localhost:5432/ai
   GEMINI_API=your-api-key
   MODEL_ID=gemini-2.0-flash
   MAX_TOKENS=16384  # Optional, defaults to 16384
   ```

3. **Initialization code**
   ```go
   // Load configuration from environment
   cfg := ai.LoadConfig()

   // Create database pool
   db, _ := pgxpool.New(ctx, cfg.DatabaseURL)

   // Create provider and store
   provider := gemini.New(cfg.GeminiAPI, cfg.ModelID)
   store := postgres.New(db)

   // Schema creation now uses migrations
   store.CreateSchema(ctx)
   ```

4. **Database setup**
   - Run schema creation in your application startup
   - Migrations apply automatically
   - First run creates both schema and migration tracking table
   - Subsequent runs verify migrations haven't changed

#### For new users:

1. **Install package**
   ```bash
   go get github.com/meikuraledutech/ai/v1
   ```

2. **Configure environment** (see above)

3. **Use the API** (see examples in `v1/example/`)

### Deprecations

None - This is the first release.

### Performance Improvements

- **Database queries optimized**
  - Index on `ai_messages.session_id` for faster message retrieval
  - Unique constraint on `(session_id, seq)` enables efficient ordered queries

- **Configuration loading optimized**
  - Single-pass environment variable parsing
  - No runtime string conversions

### Security

- **Token checksum verification** - Migrations validated to detect tampering
- **Transactional schema updates** - Database consistency guaranteed
- **Environment variable isolation** - Sensitive data not stored in code or defaults

### Testing

All functionality tested with:
- ✅ Schema creation and migration application
- ✅ Multi-turn conversation storage and retrieval
- ✅ Large prompt handling up to token limits
- ✅ Token usage tracking and accuracy
- ✅ Message ordering via sequence numbers
- ✅ Conversation history preservation
- ✅ Environment variable configuration

### Known Limitations

- Single database provider (PostgreSQL) - other databases require custom Store implementation
- Single LLM provider (Gemini) - other providers require custom Provider implementation
- Token limit enforcement at API level - client-side estimation recommended for large requests

### Future Plans

- **v1.1.0**: Streaming responses for large outputs
- **v1.2.0**: Response truncation detection and warnings
- **v1.3.0**: Token pre-estimation before sending requests
- **v2.0.0**: Additional database providers, additional LLM providers, plugin architecture

---

## Legend

- **Added** - New functionality
- **Changed** - Changes in existing functionality
- **Removed** - Removed functionality
- **Fixed** - Bug fixes
- **Deprecated** - Soon-to-be removed features
- **Security** - Security improvements and fixes

For more information, see the [README.md](README.md) and [v1/DOCS.md](v1/DOCS.md).
