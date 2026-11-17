# Pull request guidelines

This document complements [`CONTRIBUTING.md`](../CONTRIBUTING.md) and the
[pull request template](../.github/PULL_REQUEST_TEMPLATE.md).

## Before you open a PR

1. Search existing issues and pull requests for duplicates.
2. Prefer a linked issue for non-trivial changes.
3. Keep the branch focused on one concern.
4. Run the full local suite:

   ```bash
   make build
   make vet
   make test-race
   make lint
   ```

5. Fill every section of the pull request template that applies.

## Size and scope

- One logical change per PR.
- Prefer small, reviewable diffs over large mixed refactors.
- Do not bundle dependency bumps with feature work unless the feature requires them.
- Do not reformat unrelated files.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```text
<type>[optional scope]: <description>

[optional body]
```

Allowed types: `feat`, `fix`, `docs`, `test`, `chore`, `refactor`, `ci`, `perf`.

Rules:

- Imperative mood in the subject (`add`, `fix`, `update`).
- No trailing period on the subject line.
- No emoji.
- No AI tool attribution and no `Co-authored-by` lines for assistants.

## Review expectations

Reviewers look for:

- Correctness and race safety in concurrent SSH / DAG paths.
- Fail-closed behavior around validation, approvals, rollback, and audit.
- Tests that cover edge cases, not only the happy path.
- Clear GoDoc on exported identifiers.
- No secrets in diffs, logs, or examples.

## Security-sensitive PRs

Call out explicitly when the change touches:

- Command parsing, interpolation, or denylist behavior
- SSH authentication, host-key verification, or transports
- Secret providers or redaction
- Approval gates
- Rollback / checkpoint / resume
- Audit log format or verification

Those PRs should explain residual risk and how it was tested.

## After merge

- Delete the branch once merged.
- Follow up with docs if user-facing behavior changed.
- For releases, use tagged `v*` pushes so GoReleaser and the Docker image publish.
