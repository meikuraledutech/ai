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
