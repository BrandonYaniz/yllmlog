# Chat Interface

The chat interface is the primary way an admin can change runtime behavior without editing database rows manually.

The LLM interprets the request and proposes structured actions. The daemon validates those actions, classifies risk, asks for confirmation where required, and applies approved changes.

## Risk classes

- Read-only.
- Safe change.
- Risky change.
- Dangerous or future correction change.

The LLM must not directly mutate SQLite or the file system.
