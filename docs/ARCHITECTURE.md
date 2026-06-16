# Architecture

yllmlog uses a daemon plus CLI model.

```text
yllmlog CLI
→ Unix domain socket
→ yllmlog daemon
→ Unix domain socket
→ yllmd
→ local model
```

The daemon owns log intake, offset tracking, rotation detection, rule evaluation, event grouping, LLM analysis, notification policy, reports, and SQLite persistence.

The CLI is a local client for inspecting status, reviewing issues, managing watched logs, editing rules, reading reports, and using the chat-based administration interface.

No TCP API is part of the default architecture.
