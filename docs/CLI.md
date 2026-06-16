# CLI

Planned command groups:

```sh
yllmlog status
yllmlog logs list
yllmlog logs add /var/log/maillog
yllmlog logs remove /var/log/maillog
yllmlog issues
yllmlog issues --immediate
yllmlog live
yllmlog rules list
yllmlog reports daily
yllmlog smtp test
yllmlog chat
```

The CLI connects to `yllmlogd` over a local Unix domain socket.
