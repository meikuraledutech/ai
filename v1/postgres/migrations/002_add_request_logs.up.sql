CREATE TABLE IF NOT EXISTS ai_request_logs (
    id                TEXT PRIMARY KEY,
    session_id        TEXT NOT NULL REFERENCES ai_sessions(id) ON DELETE CASCADE,
    prompt            TEXT NOT NULL,
    response          TEXT NOT NULL DEFAULT '',
    attempt_number    INT NOT NULL DEFAULT 1,
    retry_count       INT NOT NULL DEFAULT 0,
    final_status      TEXT NOT NULL DEFAULT 'pending',
    fail_reason       TEXT NOT NULL DEFAULT '',
    error_message     TEXT NOT NULL DEFAULT '',
    prompt_tokens     INT NOT NULL DEFAULT 0,
    response_tokens   INT NOT NULL DEFAULT 0,
    total_tokens      INT NOT NULL DEFAULT 0,
    thought_tokens    INT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_request_logs_session ON ai_request_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_ai_request_logs_status  ON ai_request_logs(final_status);
