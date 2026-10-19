---
name: oom-mitigation
version: "1.0"
service: api-worker
severity: critical
author: sre-team
targets:
  - group: workers
vars:
  service_name:
    type: string
    default: api-worker
  memory_threshold_percent:
    type: int
    default: 90
approval:
  mode: per_step
  timeout: 5m
---

## Step: check-memory - Check current memory pressure
read_only: true

Read-only check: reports current memory usage so the operator can confirm this is
actually a memory pressure incident before restarting anything.

```bash opsrun:check expect_exit=0
free -m | awk '/Mem:/ {printf "%d\n", $3*100/$2}' | awk -v t={{ .Vars.memory_threshold_percent }} '{exit ($1 < t)}'
```

## Step: identify-top-consumer - Identify the top memory consumer
depends_on: [check-memory]
read_only: true

Logs the heaviest processes by resident memory, for the post-mortem, before taking any
action.

```bash opsrun:run target=workers timeout=10s
ps -eo pid,comm,%mem --sort=-%mem | head -10
```

## Step: restart-worker - Restart the worker service
depends_on: [identify-top-consumer]
confirm: true
retries: 1

Restarts the worker service to release leaked memory immediately.

```bash opsrun:run target=workers timeout=45s
systemctl restart {{ .Vars.service_name }}
```

```bash opsrun:check expect_exit=0
systemctl is-active {{ .Vars.service_name }}
```

```bash opsrun:rollback
systemctl restart {{ .Vars.service_name }}
```

## Step: verify-memory-recovered - Verify memory pressure has subsided
depends_on: [restart-worker]
read_only: true

```bash opsrun:check expect_exit=0
free -m | awk '/Mem:/ {printf "%d\n", $3*100/$2}' | awk -v t={{ .Vars.memory_threshold_percent }} '{exit ($1 >= t)}'
```
