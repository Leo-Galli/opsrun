---
name: rolling-application-deploy
version: "1.0"
service: web-frontend
severity: medium
author: sre-team
targets:
  - group: web
vars:
  release_tag:
    type: string
    required: true
  health_endpoint:
    type: string
    default: "http://localhost:8080/healthz"
approval:
  mode: per_step
  timeout: 10m
---

Run this runbook with a host rollout strategy so the deploy touches hosts gradually
instead of all at once, for example:

```
opsrun run examples/runbooks/rolling-deploy.md --var release_tag=v1.4.2 \
  --strategy rolling --batch-size 2 --max-failure-percent 20
```

or, for a canary release that deploys to a single host first:

```
opsrun run examples/runbooks/rolling-deploy.md --var release_tag=v1.4.2 \
  --strategy canary --canary-count 1
```

## Step: pull-release - Pull the release artifact
confirm: true

```bash opsrun:run target=web timeout=2m
docker pull registry.internal/web-frontend:{{ .Vars.release_tag }}
```

## Step: deploy-release - Deploy the new release
depends_on: [pull-release]
confirm: true

```bash opsrun:run target=web timeout=1m
docker-compose -f /opt/web-frontend/docker-compose.yml up -d --no-deps web-frontend
```

```bash opsrun:check expect_exit=0
curl -fsS {{ .Vars.health_endpoint }}
```

```bash opsrun:rollback
docker-compose -f /opt/web-frontend/docker-compose.yml up -d --no-deps web-frontend
```

## Step: verify-version - Verify the deployed version matches the release
depends_on: [deploy-release]
read_only: true

```bash opsrun:check expect_exit=0
docker exec web-frontend cat /app/VERSION | grep -q {{ .Vars.release_tag }}
```
