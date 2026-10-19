---
name: full-disk-remediation
version: "1.0"
service: web-frontend
severity: high
author: sre-team
targets:
  - group: web
vars:
  cache_dir:
    type: string
    default: "/var/cache/web-frontend"
    required: false
approval:
  mode: per_step
  timeout: 5m
---

## Step: check-disk - Check disk usage

Reports current disk usage on the root filesystem so the operator can confirm the
incident before anything is changed.

```bash opsrun:check expect_exit=0
df -h / | tail -1
```

## Step: clear-cache - Clear application cache
depends_on: [check-disk]
confirm: true

Removes the contents of the application cache directory. This is safe to repeat and
does not require a rollback because the cache is rebuilt automatically on first use.

```bash opsrun:run target=web timeout=30s
rm -rf {{ .Vars.cache_dir }}/*
```

```bash opsrun:rollback
echo "cache directory cleared; nothing to restore"
```

## Step: restart-service - Restart the application service
depends_on: [clear-cache]

Restarts the service so it picks up the freed disk space, then confirms it came back
healthy.

```bash opsrun:run target=web timeout=60s
systemctl restart web-frontend
```

```bash opsrun:check expect_exit=0
systemctl is-active web-frontend
```

```bash opsrun:rollback
systemctl restart web-frontend
```
