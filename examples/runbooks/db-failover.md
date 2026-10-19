---
name: database-failover
version: "1.0"
service: orders-db
severity: critical
author: sre-team
targets:
  - group: db
vars:
  replica_host:
    type: string
    required: true
  primary_host:
    type: string
    required: true
approval:
  mode: all
  timeout: 15m
---

## Step: check-replica-lag - Confirm the replica is caught up
read_only: true

Fails closed: refuses to proceed with a failover onto a replica that has not caught up
with the primary, which would lose committed transactions.

```bash opsrun:check expect_exit=0
pg_replica_lag_seconds --host {{ .Vars.replica_host }} --max-lag 5
```

## Step: promote-replica - Promote the replica to primary
depends_on: [check-replica-lag]
confirm: true

Promotes the replica out of recovery mode so it starts accepting writes.

```bash opsrun:run target=db timeout=60s
pg_ctl promote -D /var/lib/postgresql/data --host {{ .Vars.replica_host }}
```

```bash opsrun:check expect_exit=0
pg_isready --host {{ .Vars.replica_host }}
```

```bash opsrun:rollback
echo "promotion cannot be safely reversed automatically; see runbook: demote-and-resync"
```

## Step: repoint-application - Repoint the application at the new primary
depends_on: [promote-replica]
confirm: true

Updates the application's connection configuration to use the newly promoted primary
and reloads it without a full restart.

```bash opsrun:run target=web timeout=30s
sed -i "s/{{ .Vars.primary_host }}/{{ .Vars.replica_host }}/" /etc/app/db.conf && systemctl reload app
```

```bash opsrun:check expect_exit=0
systemctl is-active app
```

```bash opsrun:rollback
sed -i "s/{{ .Vars.replica_host }}/{{ .Vars.primary_host }}/" /etc/app/db.conf && systemctl reload app
```
