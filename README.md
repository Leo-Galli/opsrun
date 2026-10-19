# OpsRun

OpsRun turns Markdown runbooks into safe, concurrent incident response workflows executed
over SSH, with a DAG-based executor, automatic rollback, human approval gates, and an
immutable audit log for post-mortems.

[![CI](https://github.com/Leo-Galli/opsrun/actions/workflows/ci.yml/badge.svg)](https://github.com/Leo-Galli/opsrun/actions/workflows/ci.yml)
[![Lint](https://github.com/Leo-Galli/opsrun/actions/workflows/lint.yml/badge.svg)](https://github.com/Leo-Galli/opsrun/actions/workflows/lint.yml)
[![CodeQL / govulncheck](https://github.com/Leo-Galli/opsrun/actions/workflows/security.yml/badge.svg)](https://github.com/Leo-Galli/opsrun/actions/workflows/security.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Leo-Galli/opsrun.svg)](https://pkg.go.dev/github.com/Leo-Galli/opsrun)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

## Why OpsRun

Incident response runbooks usually live as wiki pages that get copy-pasted into terminals
by hand, one host at a time, under pressure. That is slow and error-prone: there is no
dependency tracking between steps, no automatic rollback when something goes wrong, no
record of who approved what, and no safe way to run the same remediation across hundreds
of hosts at once.

OpsRun treats a runbook as executable, auditable code:

- Runbooks are plain Markdown with a small set of structured conventions, so they stay
  readable and reviewable like any other document, and diffable like any other file in
  version control.
- Steps form a DAG. Independent steps run concurrently, dependent steps wait, and you
  control host rollout with all-at-once, rolling-batch, or canary strategies.
- Every destructive step can declare a rollback action. If a step fails, OpsRun
  automatically rolls back the steps that already succeeded, in reverse dependency order.
- Steps can require human approval before running, showing the interpolated command and
  the exact target hosts before anything executes.
- Every command, approval, and result is written to an append-only, hash-chained audit
  log, so a post-mortem has a tamper-evident record of exactly what happened.

## Status

OpsRun is under active development, built in phases. The current phase and detailed
progress are tracked in the repository's project board and `CHANGELOG.md`. The design
described in this README reflects the target v1 behavior; see `CHANGELOG.md` for what is
implemented in the current version.

## Installation

### Prebuilt binaries

Download the binary for your platform from the
[Releases](https://github.com/Leo-Galli/opsrun/releases) page, verify the checksum, and
place it on your `PATH`:

```bash
curl -LO https://github.com/Leo-Galli/opsrun/releases/latest/download/opsrun_linux_amd64.tar.gz
curl -LO https://github.com/Leo-Galli/opsrun/releases/latest/download/checksums.txt
sha256sum --ignore-missing -c checksums.txt
tar -xzf opsrun_linux_amd64.tar.gz
sudo install -m 0755 opsrun /usr/local/bin/opsrun
```

### go install

```bash
go install github.com/Leo-Galli/opsrun/cmd/opsrun@latest
```

### Docker

```bash
docker run --rm -v "$PWD":/work -w /work \
  -v "$HOME/.ssh":/root/.ssh:ro \
  ghcr.io/leo-galli/opsrun:latest run runbook.md --dry-run
```

## Quick start: a real incident example

Imagine a disk-space incident on a group of web servers. The runbook below checks disk
usage, clears a known-safe cache directory if the disk is over a threshold, and restarts
the application service, with a rollback that restores the service if the restart fails
health checks.

`examples/runbooks/full-disk.md` (abridged):

```markdown
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
approval:
  mode: per_step
---

## Step: check-disk - Check disk usage

```bash opsrun:check expect_exit=0
df -h / | tail -1
```

## Step: clear-cache - Clear application cache
depends_on: [check-disk]
confirm: true

```bash opsrun:run target=web timeout=30s
rm -rf {{ .Vars.cache_dir }}/*
```

```bash opsrun:rollback
echo "cache directory cleared; nothing to restore"
```

## Step: restart-service - Restart the application service
depends_on: [clear-cache]

```bash opsrun:run target=web timeout=60s
systemctl restart web-frontend
```

```bash opsrun:check expect_exit=0
systemctl is-active web-frontend
```

```bash opsrun:rollback
systemctl restart web-frontend
```
```

Validate the runbook and inspect the execution plan without touching any host:

```bash
opsrun validate examples/runbooks/full-disk.md
opsrun plan examples/runbooks/full-disk.md --vars-file prod.vars.yaml
```

Run it for real, with per-step approval prompts and automatic rollback on failure:

```bash
opsrun run examples/runbooks/full-disk.md \
  --config opsrun.yaml \
  --vars-file prod.vars.yaml \
  --interactive
```

If the run is interrupted (network loss, Ctrl+C, SIGTERM), resume it from its last
checkpoint:

```bash
opsrun resume <run-id>
```

After the incident, export a post-mortem timeline straight from the tamper-evident audit
log:

```bash
opsrun audit verify <run-id>
opsrun audit export <run-id> --format markdown > postmortem.md
```

More complete examples live in `examples/runbooks/`: a service restart with health check,
a full-disk remediation, and a database failover.

## Runbook format

Runbooks are standard Markdown with YAML front matter for metadata and a small set of
conventions for steps:

- YAML front matter declares the runbook's name, version, target service, severity,
  author, target hosts or groups, required variables (with type, default, and
  validation), and the approval policy.
- Each step is a level-2 heading: `## Step: <id> - <title>`, followed by prose
  description and fenced code blocks with a structured info string:
  - ` ```bash opsrun:run target=<group> timeout=<duration> ` for the action to execute.
  - ` ```bash opsrun:check expect_exit=<code> ` for a post-condition check.
  - ` ```bash opsrun:rollback ` for the compensating action if a later step fails.
  - Step directives (`depends_on`, `confirm`, `retries`, `continue_on_error`, parallel or
    serial execution, read-only diagnostic steps) are declared next to the heading.
- Variables are referenced with `{{ .Vars.name }}` and are safely shell-escaped on
  interpolation; referencing an undeclared variable is a validation error, not a runtime
  surprise.

The full grammar, validation rules (cycle detection, missing rollback on destructive
steps, dangerous command denylist, and more), and worked examples are documented in
[`docs/runbook-spec.md`](docs/runbook-spec.md).

## Architecture

OpsRun parses a runbook into a domain model with a goldmark-based Markdown AST walker
(never regex on raw text), builds a dependency DAG from the steps, and executes it
through a pluggable `Transport`/`Session` abstraction, with SSH as the primary transport
and a `local` transport for the control node and for tests. Execution state transitions
through a documented state machine per step and per host, with approval gates, automatic
rollback, and checkpointing for resume. See [`docs/architecture.md`](docs/architecture.md)
for the full component diagram and execution flow.

## Security model

- No command runs without first being parsed, validated, interpolated, and shown to the
  operator; `--dry-run` is always available to preview a run with no side effects.
- SSH host key verification against `known_hosts` is mandatory; disabling it requires an
  explicit flag and prints a loud warning. It is never silently bypassed.
- Secrets (passphrases, private keys, vault tokens) are resolved through a
  `SecretProvider` abstraction and are never written to logs; the audit log redacts
  secret values from interpolated commands before they are persisted.
- The audit log is append-only and hash-chained (each record embeds the SHA-256 of the
  previous one), with optional Ed25519 signing, so tampering after the fact is
  detectable with `opsrun audit verify`.
- Destructive steps are expected to declare a rollback action; the validator flags steps
  that look destructive (matched against a configurable denylist) but have none.
- OpsRun follows the principle of least privilege: it does not require a persistent agent
  or elevated daemon on target hosts, only standard SSH access and, optionally,
  non-interactive `sudo` for specific commands.

Please report suspected security vulnerabilities following the process in
[`SECURITY.md`](SECURITY.md) rather than opening a public issue.

## Configuration

OpsRun is configured through `opsrun.yaml` (host inventory and groups, users, keys, jump
hosts, concurrency limits, timeouts, audit log path, approval policy), overridable by
environment variables and command-line flags. See
[`docs/architecture.md`](docs/architecture.md) and the sample `opsrun.yaml` in the
repository root for the full schema.

## Development

```bash
make build   # build the opsrun binary into ./bin
make test    # go test -race ./...
make lint    # golangci-lint run
make vuln    # govulncheck ./...
```

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the contribution workflow, coding
conventions, and commit message format.

## Roadmap

- Phase 1: core architecture, domain interfaces, configuration, project tooling.
- Phase 2: Markdown AST runbook parser and validator.
- Phase 3: concurrent SSH worker pool with pooling, retries, and streaming output.
- Phase 4: DAG execution engine, rollback, approval gates, checkpoint/resume.
- Phase 5: Cobra CLI, Bubble Tea TUI, hash-chained audit log.
- Phase 6: v0.1.0 release with GoReleaser, SBOM, and multi-platform binaries.

Beyond v1: two-person approval for critical steps, a Vault-backed `SecretProvider`,
syslog/external `AuditSink` implementations, and SSH certificate authority integration.

## License

Apache License 2.0. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).
