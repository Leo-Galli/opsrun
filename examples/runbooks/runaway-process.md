---
name: runaway-process-mitigation
version: "1.0"
service: batch-worker
severity: high
author: sre-team
targets:
  - group: workers
vars:
  process_name:
    type: string
    required: true
  cpu_threshold_percent:
    type: int
    default: 95
approval:
  mode: per_step
  timeout: 5m
---

## Step: identify-process - Identify the runaway process
read_only: true

Read-only: lists every process matching the name and its current CPU usage, so the
operator can confirm this is the right process before killing anything.

```bash opsrun:run target=workers timeout=10s
ps -eo pid,comm,%cpu --sort=-%cpu | grep {{ .Vars.process_name }} | head -5
```

## Step: attempt-graceful-stop - Attempt a graceful stop first
depends_on: [identify-process]
confirm: true
retries: 1

```bash opsrun:run target=workers timeout=15s
pkill -TERM -f {{ .Vars.process_name }}
```

```bash opsrun:check expect_exit=1
pgrep -f {{ .Vars.process_name }}
```

## Step: force-kill - Force kill if it is still running
depends_on: [attempt-graceful-stop]
confirm: true
continue_on_error: false

Only reached if the graceful stop above did not work; force-kills the process.

```bash opsrun:run target=workers timeout=10s
pkill -KILL -f {{ .Vars.process_name }}
```

```bash opsrun:check expect_exit=1
pgrep -f {{ .Vars.process_name }}
```

```bash opsrun:rollback
echo "a killed process cannot be un-killed; if it is managed by systemd, restart its unit directly"
```

## Step: verify-cpu-recovered - Verify CPU load has recovered
depends_on: [force-kill]
read_only: true

```bash opsrun:check expect_exit=0
top -bn1 | awk '/Cpu\(s\)/ {split($2,a,"%"); exit (a[1] >= {{ .Vars.cpu_threshold_percent }})}'
```
