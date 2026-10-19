# Changelog

All notable changes to this project are documented in this file. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **Project scaffolding**: standard Go layout, core domain and interface definitions
  (`RunbookParser`, `Runbook`/`Step`/`Variable`/`Action`, `Inventory`, `Transport`/
  `Session`, `Executor`, `DAG`, `ApprovalGate`, `RollbackManager`, `AuditSink`,
  `SecretProvider`), `opsrun.yaml` configuration schema and loader, project tooling
  (Makefile, golangci-lint v2 config, GoReleaser v2 config, multi-stage Dockerfile), and
  CI workflows for linting, testing, multi-platform builds, and vulnerability scanning.
- **Markdown runbook parser and validator** (`internal/parser`): a goldmark-based AST
  walker (never regex on raw document text) producing the domain model with precise
  line/column positions; strict, unknown-fields-rejecting YAML front matter; a directive
  block grammar for `depends_on`, `confirm`, `retries`, `continue_on_error`, `mode`,
  `read_only`, and per-step approval overrides; `{{ .Vars.name }}` interpolation with
  mandatory shell escaping and rejection of undeclared variables; a validator catching
  duplicate step ids, unknown or cyclic dependencies, undeclared variables, a
  configurable dangerous-command denylist, and destructive steps with no rollback
  action. Documented in `docs/runbook-spec.md`.
- **Concurrent SSH worker pool** (`internal/ssh`): a per-host connection pool with
  global and per-host concurrency limits, ssh-agent / private-key (including
  passphrase-encrypted keys) / SSH-certificate authentication, `ProxyJump` support, a
  hand-written `~/.ssh/config` parser, mandatory `known_hosts` verification, exponential
  backoff with jitter for retryable network errors only, host-prefixed streaming
  output, non-interactive `sudo`, optional PTY allocation, and a `local` transport for
  control-node commands and tests. A scriptable `FakeTransport` supports fast,
  deterministic tests of everything built on top of `Transport`/`Session`.
- **DAG execution engine** (`internal/executor`): topological execution with
  independent steps in the same DAG level run concurrently, bounded by
  `--concurrency`; host rollout strategies (all-at-once, rolling batches with a
  tolerated failure percentage, canary); an explicit, validated step/host state
  machine; human-in-the-loop approval (approve / skip / abort / edit a variable and
  re-prompt); automatic reverse-order rollback of completed steps on failure, with its
  own timeout and no retries on a failing rollback; on-disk checkpoints and
  `opsrun resume` to continue an interrupted or failed run without re-executing
  already-succeeded steps.
- **Hash-chained audit log** (`internal/audit`): append-only JSON Lines, each record
  embedding the SHA-256 hash of the previous one, resumable across process restarts,
  optional Ed25519 signing, `opsrun audit verify` reporting every tampering violation
  found (not just the first), and a Markdown post-mortem export.
- **Secret redaction**: runbook variables declared `secret: true` are interpolated with
  their real value only in the command actually sent to a host; every other surface —
  approval prompts, the TUI, emitted events, the audit log — sees `***REDACTED***`
  instead.
- **Cobra CLI** (`cmd/opsrun`): `run`, `validate`, `plan`, `resume`,
  `audit {list,show,verify,export,keygen}`, `hosts`, `version`, `completion`. Global
  flags `--config`, `--var`, `--vars-file`, `--dry-run`, `--interactive`,
  `--non-interactive`, `--concurrency`, `--output text|json`,
  `--insecure-skip-host-key-verification` (loud warning, opt-in only). Rollout
  strategies are selectable per run with `--strategy all-at-once|rolling|canary`,
  `--batch-size`, `--max-failure-percent`, and `--canary-count`. Documented exit codes.
- **Bubble Tea TUI** (`internal/tui`): a live DAG/step status view, per-host state,
  streaming command log, and interactive approval prompts, opened automatically when
  stdout is a real terminal and falling back automatically to plain, line-oriented,
  emoji-free text output otherwise (CI, pipes, `--output json`).
- **Nine example runbooks** in `examples/runbooks/`, each validated and plan-tested as
  part of the automated test suite: full-disk remediation, service restart with health
  check, database failover, TLS certificate renewal, OOM mitigation, log/disk cleanup,
  load-balancer node drain (demonstrating `secret: true`), a rolling/canary deploy, and
  runaway-process mitigation.

### Fixed

- A connection-pool race: concurrent `Open()` calls for a host with no pooled
  connection yet could each observe "not found" and all dial, leaking connections and
  exhausting the global connection semaphore under load. Fixed with a per-host
  single-flight dial.
- Rollback was running on the same context a run had just been canceled on, so
  `Session.Run` refused to execute the rollback command — meaning an operator's
  Ctrl+C would have made rollback impossible. Fixed by running rollback on a context
  detached from the parent's cancellation (`context.WithoutCancel`).
- The SIGINT/SIGTERM handler printed "interrupt received" on every run, not just real
  interrupts, because it watched a context whose cancel function also ran at the end of
  a normal run. Fixed by watching the raw OS signal channel directly.
- TTY detection for choosing the TUI used a stdlib-only heuristic that cannot tell a
  real terminal apart from `/dev/null` (both are character devices), which made
  `opsrun run ... >/dev/null 2>&1` hang waiting for a terminal that was never there.
  Fixed with `golang.org/x/term.IsTerminal`.
