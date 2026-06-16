# Configuration

The file configuration is intentionally minimal. Most runtime configuration lives in SQLite.

## Example

```yaml
data_dir: /var/db/yllmlog
yllmd:
  socket: /var/run/yllmd/yllmd.sock
  profile: phi
daemon:
  socket: /var/run/yllmlog/yllmlog.sock
```

The database stores watched logs, services, rules, notification settings, SMTP settings, report schedules, admin preferences, chat actions, and future correction workflow state.
