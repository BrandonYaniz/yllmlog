# yllmlog

yllmlog is a local-first Go daemon and CLI for turning system logs into useful administrator guidance. It watches configured logs, learns which lines matter, groups repeated warnings and errors into issue records, explains them in plain English through a local LLM interface, and reports what the admin needs to know through the CLI and email.

The project is designed for FreeBSD first, then macOS, then Linux.

## Design goals

- Local-only operation.
- Daemon plus CLI architecture.
- Unix domain socket communication.
- SQLite-backed runtime state.
- Minimal file-based configuration.
- Local model access through yllmd.
- Human-readable issue summaries.
- Rule performance tracking.
- Non-destructive future correction workflows.

## Commands

The project builds two commands:

```sh
yllmlog   # CLI client
yllmlogd  # long-running daemon
```

## Minimal configuration

Most settings live in SQLite so they can be changed through the CLI and chat interface. The file configuration only provides enough information to find the data directory and local model interface.

```yaml
data_dir: /var/db/yllmlog
yllmd:
  socket: /var/run/yllmd/yllmd.sock
  profile: phi
daemon:
  socket: /var/run/yllmlog/yllmlog.sock
```

## Development status

This repository currently contains the initial project skeleton, public documentation, and private implementation planning files under `.codex/`. The `.codex/` directory is ignored by Git and is not part of the public project history.
