# Security

yllmlog is designed as a local administration tool. It should not expose a public network service.

## Local-only design

The CLI communicates with the daemon over a local Unix domain socket. The daemon communicates with yllmd over a local Unix domain socket. TCP listeners are not part of the default design.

## LLM boundary

The LLM proposes analysis, rules, summaries, and structured actions. The daemon validates actions before writing to SQLite or changing runtime behavior. The LLM must not directly mutate the database or file system.

## Future correction safety

Future correction workflows must be non-destructive. Files are never permanently deleted. A delete-like operation moves the file into a managed backup area. File modifications require a backup before the change is applied. Restores require backing up the current version before replacement.

## Reporting issues

Report security issues privately to the project maintainer.
