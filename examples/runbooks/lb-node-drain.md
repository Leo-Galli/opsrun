---
name: lb-node-drain-for-maintenance
version: "1.0"
service: web-frontend
severity: medium
author: sre-team
targets:
  - group: web
vars:
  lb_api_token:
    type: string
    required: true
    secret: true
  lb_api_url:
    type: string
    default: "https://lb-admin.internal/api/v1"
  drain_timeout_seconds:
    type: int
    default: 30
approval:
  mode: per_step
  timeout: 10m
---

## Step: drain-from-lb - Remove the host from the load balancer rotation
confirm: true

Deregisters the host from the load balancer and waits for in-flight connections to
drain, so maintenance can proceed without dropping live traffic.

```bash opsrun:run target=web timeout=1m
curl -fsS -X POST -H "Authorization: Bearer {{ .Vars.lb_api_token }}" "{{ .Vars.lb_api_url }}/drain" -d "host=$(hostname)&timeout={{ .Vars.drain_timeout_seconds }}"
```

```bash opsrun:check expect_exit=0
curl -fsS -H "Authorization: Bearer {{ .Vars.lb_api_token }}" "{{ .Vars.lb_api_url }}/status?host=$(hostname)" | grep -q '"state":"drained"'
```

```bash opsrun:rollback
curl -fsS -X POST -H "Authorization: Bearer {{ .Vars.lb_api_token }}" "{{ .Vars.lb_api_url }}/rejoin" -d "host=$(hostname)"
```

## Step: perform-maintenance - Run the maintenance task
depends_on: [drain-from-lb]
confirm: true

Placeholder for the actual maintenance action (patching, a config change, a service
restart); replace this command with the real maintenance step for your incident.

```bash opsrun:run target=web timeout=5m
systemctl restart web-frontend
```

```bash opsrun:check expect_exit=0
systemctl is-active web-frontend
```

## Step: rejoin-lb - Rejoin the load balancer rotation
depends_on: [perform-maintenance]

```bash opsrun:run target=web timeout=1m
curl -fsS -X POST -H "Authorization: Bearer {{ .Vars.lb_api_token }}" "{{ .Vars.lb_api_url }}/rejoin" -d "host=$(hostname)"
```

```bash opsrun:check expect_exit=0
curl -fsS -H "Authorization: Bearer {{ .Vars.lb_api_token }}" "{{ .Vars.lb_api_url }}/status?host=$(hostname)" | grep -q '"state":"active"'
```
