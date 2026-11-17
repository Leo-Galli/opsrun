# Issue guidelines

Use the GitHub issue forms under `.github/ISSUE_TEMPLATE/`. Blank issues are disabled.

## Choose the right template

| Template | Use when |
|---|---|
| Bug report | Something is broken or incorrect |
| Feature request | You want new behavior that fits OpsRun safety rules |
| Documentation | Docs are missing, unclear, or wrong |

For suspected vulnerabilities, follow [`SECURITY.md`](../SECURITY.md). Do not open a public issue.

## Writing a good bug report

- Give a short summary and exact reproduction steps.
- Paste full command output with secrets and sensitive hostnames redacted.
- Include `opsrun version`, OS/arch, and a minimal runbook when possible.
- Prefer failing cases that work with `--dry-run` or the example runbooks when feasible.

## Writing a good feature request

- Start from the user problem, not an implementation.
- Explain safety impact on validation, approvals, rollback, SSH host keys, and audit.
- Confirm the request does not bypass fail-closed checks or log secrets.

## Labels and titles

Issue forms set a conventional title prefix (`[bug]:`, `[feat]:`, `[docs]:`) and a label.
Keep that prefix so triage stays consistent.
