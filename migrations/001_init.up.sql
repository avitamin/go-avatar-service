CREATE TABLE IF NOT EXISTS avatars (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    file_name TEXT NOT NULL,
    original_mime_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    original_key TEXT NOT NULL,
    thumb100_key TEXT NOT NULL DEFAULT '',
    thumb300_key TEXT NOT NULL DEFAULT '',
    original_available BOOLEAN NOT NULL DEFAULT TRUE,
    thumb100_available BOOLEAN NOT NULL DEFAULT FALSE,
    thumb300_available BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL CHECK (status IN ('processing', 'completed', 'failed')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS avatars_user_created_idx
    ON avatars (user_id, created_at DESC)
    WHERE deleted_at IS NULL;
