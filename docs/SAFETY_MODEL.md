# Safety Model

yllmlog is a local-first administration tool.

## Boundaries

- No default TCP listener.
- Local socket communication only.
- Local model access through yllmd.
- SQLite for runtime state.
- LLM suggestions validated by daemon logic.
- Dangerous changes require explicit confirmation.

## Future corrections

Future correction capabilities must be supervised, reversible, and non-destructive.
