---
name: service-restart-with-healthcheck
version: "1.0"
service: payments-api
severity: medium
author: sre-team
targets:
  - group: payments
vars:
  service_name:
    type: string
    default: payments-api
    required: false
  health_endpoint:
    type: string
    default: "http://localhost:8080/healthz"
    required: false
approval:
  mode: per_step
  timeout: 10m
---

## Step: pre-check - Confirm the service is currently unhealthy
read_only: true

Read-only check run before touching anything, to confirm the incident is real and to
have a baseline in the audit log.

```bash opsrun:check expect_exit=0
systemctl status {{ .Vars.service_name }} --no-pager
```

## Step: restart - Restart the service
depends_on: [pre-check]
confirm: true
retries: 1

Restarts the service and waits for systemd to report it active.

```bash opsrun:run target=payments timeout=45s
systemctl restart {{ .Vars.service_name }}
```

```bash opsrun:check expect_exit=0
systemctl is-active {{ .Vars.service_name }}
```

```bash opsrun:rollback
systemctl restart {{ .Vars.service_name }}
```

## Step: verify-health - Verify the health endpoint
depends_on: [restart]

Confirms the application-level health check passes, not just that the process is
running.

```bash opsrun:check expect_exit=0
curl -fsS {{ .Vars.health_endpoint }}
```
