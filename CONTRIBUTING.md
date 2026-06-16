# Contributing

yllmlog is a Go system administration project. Changes should be direct, testable, and conservative around file system, socket, and database behavior.

## Expectations

- Keep packages small and focused.
- Prefer explicit errors over hidden behavior.
- Add tests for path handling, socket behavior, database migrations, rules, and log rotation.
- Treat file operations as safety-critical.
- Do not introduce a remote listener without a deliberate design review.

## Comments

Comments should explain intent, edge cases, safety constraints, and non-obvious decisions. Avoid comments that narrate obvious code behavior.

## Commit messages

Use direct, specific commit messages, such as:

```text
Add SQLite schema for watched logs
Track file offsets across daemon restarts
Detect log rotation by inode and file size
Validate backup paths before file operations
```
