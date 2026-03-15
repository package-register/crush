-- Add memories table for long-term memory storage
-- +goose Up
CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    memory_data TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_memories_user ON memories(app_name, user_id);
CREATE INDEX IF NOT EXISTS idx_memories_deleted ON memories(deleted_at);

-- +goose Down
DROP INDEX IF EXISTS idx_memories_deleted;
DROP INDEX IF EXISTS idx_memories_user;
DROP TABLE IF EXISTS memories;
