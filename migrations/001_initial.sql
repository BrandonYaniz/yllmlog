-- Initial SQLite schema placeholder.
-- Detailed tables are specified in .codex/DATA_MODEL_SPEC.md.

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
