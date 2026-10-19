---
name: log-disk-cleanup
version: "1.0"
service: api-gateway
severity: medium
author: sre-team
targets:
  - group: gateway
vars:
  log_dir:
    type: string
    default: /var/log/api-gateway
  retention_days:
    type: int
    default: 7
approval:
  mode: per_step
  timeout: 10m
---

## Step: check-disk - Check disk usage before cleanup
read_only: true

```bash opsrun:check expect_exit=0
df -h /var/log | tail -1
```

## Step: compress-old-logs - Compress logs older than the retention window
depends_on: [check-disk]
continue_on_error: true

Compressing is safe to retry and safe to skip if it fails on a given host: the next
step's deletion only ever targets files already past retention, compressed or not.

```bash opsrun:run target=gateway timeout=2m
find {{ .Vars.log_dir }} -name "*.log" -mtime +{{ .Vars.retention_days }} ! -name "*.gz" -exec gzip {} \;
```

```bash opsrun:rollback
echo "compression is not reversible and does not need to be undone"
```

## Step: delete-expired-logs - Delete compressed logs past double the retention window
depends_on: [compress-old-logs]
confirm: true

```bash opsrun:run target=gateway timeout=2m
find {{ .Vars.log_dir }} -name "*.log.gz" -mtime +{{ .Vars.retention_days }} -delete
```

```bash opsrun:rollback
echo "deleted log files cannot be restored; rollback is a no-op by design"
```

## Step: verify-disk-freed - Verify disk usage improved
depends_on: [delete-expired-logs]
read_only: true

```bash opsrun:check expect_exit=0
df -h /var/log | tail -1
```
