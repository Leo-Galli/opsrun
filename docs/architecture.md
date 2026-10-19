# Architecture

OpsRun turns a Markdown runbook into a validated DAG of steps, then executes that DAG
against remote hosts through a pluggable transport, with approval gates, automatic
rollback, and an immutable audit trail.

## Components

```mermaid
flowchart TB
    subgraph Input
        RB[Markdown Runbook]
        CFG[opsrun.yaml]
        VARS[--var / --vars-file]
    end

    subgraph Core
        PARSER[internal/parser\nRunbookParser]
        MODEL[internal/runbook\nDomain model]
        VALIDATOR[internal/parser\nValidator]
        DAGPKG[internal/dag\nDAG]
        EXEC[internal/executor\nExecutor]
        APPROVAL[internal/approval\nApprovalGate]
        ROLLBACK[internal/rollback\nManager]
        INV[internal/inventory\nInventory]
        SECRETS[internal/secrets\nSecretProvider]
        AUDIT[internal/audit\nAuditSink]
    end

    subgraph Transport
        TRANSPORT[internal/ssh\nTransport / Session]
        SSHT[SSH transport]
        LOCALT[local transport]
    end

    subgraph Interface
        CLI[cmd/opsrun\nCobra CLI]
        TUI[internal/tui\nBubble Tea TUI]
    end

    RB --> PARSER --> MODEL --> VALIDATOR --> DAGPKG
    CFG --> INV
    CFG --> SECRETS
    VARS --> EXEC
    DAGPKG --> EXEC
    EXEC --> APPROVAL
    EXEC --> ROLLBACK
    EXEC --> TRANSPORT
    INV --> EXEC
    SECRETS --> TRANSPORT
    TRANSPORT --> SSHT
    TRANSPORT --> LOCALT
    EXEC --> AUDIT
    APPROVAL --> AUDIT
    ROLLBACK --> AUDIT
    CLI --> EXEC
    CLI --> TUI
    TUI --> EXEC
```

## Execution flow

```mermaid
sequenceDiagram
    participant User
    participant CLI as opsrun run
    participant Parser
    participant DAG
    participant Executor
    participant Approval as ApprovalGate
    participant Transport
    participant Audit

    User->>CLI: opsrun run runbook.md --interactive
    CLI->>Parser: Parse(runbook.md)
    Parser-->>CLI: Runbook
    CLI->>Parser: Validate(Runbook)
    Parser-->>CLI: []ValidationError
    CLI->>DAG: Build(Runbook.Steps)
    DAG-->>CLI: DAG
    CLI->>Executor: Run(ctx, Runbook, DAG, opts)
    loop for each ready batch
        Executor->>Approval: RequestApproval(step)
        Approval-->>Executor: Decision
        alt approved
            Executor->>Transport: Open + Run(step.Run)
            Transport-->>Executor: Result
            Executor->>Transport: Run(step.Checks)
            Executor->>Audit: Write(step_finished)
        else failed
            Executor->>Executor: mark step failed
            Executor->>Executor: RollbackManager.Rollback(completed)
            Executor->>Audit: Write(rollback_result)
        end
    end
    Executor-->>CLI: Summary
    CLI-->>User: exit code + summary
```

## State machine

Each step has an independent state per target host. Valid transitions:

```mermaid
stateDiagram-v2
    [*] --> pending
    pending --> waiting_approval
    pending --> running
    waiting_approval --> running: approved
    waiting_approval --> skipped: skip
    waiting_approval --> [*]: abort
    running --> succeeded
    running --> failed
    succeeded --> [*]
    failed --> rolling_back
    failed --> [*]: continue_on_error
    rolling_back --> rolled_back
    rolling_back --> rollback_failed
    rolled_back --> [*]
    rollback_failed --> [*]
    skipped --> [*]
```

The full set of transitions, including checkpointing for `opsrun resume`, is implemented
and tested in Phase 4 (`internal/executor`).

## Package boundaries

- `internal/runbook`: the domain model only. No I/O, no dependencies on other `internal`
  packages except the standard library.
- `internal/parser`: turns Markdown into `internal/runbook` values, and validates them.
  Depends only on `internal/runbook` and goldmark.
- `internal/inventory`: resolves host and group names to connection parameters. No
  dependency on transport or execution.
- `internal/ssh`: the `Transport`/`Session` abstraction, the SSH implementation, and the
  local implementation. Depends on `internal/inventory` for host parameters only.
- `internal/dag`: pure graph algorithms over `internal/runbook.Step`. No I/O.
- `internal/executor`: orchestrates `internal/dag`, `internal/ssh`, `internal/approval`,
  `internal/rollback`, and `internal/audit` to run a validated runbook.
- `internal/approval`, `internal/rollback`, `internal/audit`, `internal/secrets`: focused,
  independently testable interfaces consumed by the executor.
- `internal/config`: parses and validates `opsrun.yaml` and translates it into the types
  the other packages need (for example `inventory.Host`).
- `internal/tui` and `cmd/opsrun`: user-facing layers; they depend on the packages above,
  never the other way around.

This keeps every package testable in isolation with fakes for its dependencies, which is
how the SSH worker pool (Phase 3) and the execution engine (Phase 4) are tested without a
real network or real remote hosts.
