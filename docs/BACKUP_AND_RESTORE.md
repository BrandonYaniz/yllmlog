# Backup and Restore

Future correction workflows are non-destructive by design.

## Rules

- Files are never permanently deleted.
- Delete-like operations move files into `backup/moved/`.
- File modifications first copy the current version into `backup/modified/`.
- Restores first copy the current version into `backup/restore/`.
- Permission-only changes record reversible metadata in `backup/permissions/`.

## Backup layout

```text
/var/db/yllmlog/backup/{operation}/{sanitized-original-path}/{filename}-{YYYYMMDD-HHMMSS}.bak
```

Example:

```text
/var/db/yllmlog/backup/modified/etc/postfix/main.cf-20260615-060454.bak
```

Each backup should have a JSON manifest with original path, backup path, owner, group, mode, checksum, timestamp, workflow ID, and reason.

The file system backup is authoritative for recovery. SQLite is the index and audit trail.
