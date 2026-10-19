---
name: full-feature-runbook
version: "2.0"
service: example-svc
severity: critical
author: sre-team
targets:
  - group: web
  - host: db-primary
vars:
  retry_count:
    type: int
    default: 3
  dry_run:
    type: bool
    default: false
  grace_period:
    type: duration
    default: 30s
  cache_dir:
    type: string
    default: "/var/cache/app"
approval:
  mode: all
  timeout: 20m
---

## Step: first - First step
read_only: true

Read-only diagnostic step with no side effects.

```bash opsrun:check expect_exit=0
echo checking
```

## Step: second - Second step
depends_on: [first]
confirm: true
retries: 2
continue_on_error: false
mode: serial
approval_mode: per_step
approval_timeout: 2m

Runs the main action, serially across hosts, with explicit approval required.

```bash opsrun:run target=web timeout=45s
echo retry={{ .Vars.retry_count }} dry_run={{ .Vars.dry_run }} cache={{ .Vars.cache_dir }} grace={{ .Vars.grace_period }}
```

```bash opsrun:check expect_exit=0
echo ok
```

```bash opsrun:rollback
echo rollback
```
