# Database

yllmlog uses SQLite for runtime state.

Important data areas:

- Settings.
- Watched logs.
- Discovered log files.
- Services.
- Rules.
- Rule versions.
- Rule performance.
- Events.
- Event occurrences.
- Analyses.
- Notifications.
- Reports.
- SMTP configuration.
- Chat sessions.
- Proposed and applied actions.
- Backup records.

The database is not the only recovery mechanism for file backups. Backup files and manifests are stored on the file system in a human-readable layout.
