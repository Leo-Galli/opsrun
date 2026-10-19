# Contributing to OpsRun

Thank you for considering a contribution to OpsRun.

## Workflow

1. Fork the repository and create a branch from `main`.
2. Make your change with tests. New or changed code should have table-driven tests
   covering edge cases, not just the happy path.
3. Run the full local check suite before opening a pull request:

   ```bash
   make build
   make vet
   make test-race
   make lint
   ```

4. Open a pull request against `main` using the provided template. Keep pull requests
   small and focused on one logical change.

## Commit messages

OpsRun uses [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <short summary>

[optional body]
```

Allowed types: `feat`, `fix`, `docs`, `test`, `chore`, `refactor`, `ci`. Each commit should
represent one logical change. Do not include AI tool attribution of any kind (no
`Co-authored-by` referencing an AI assistant, no mention of any AI tool) in commit
messages, code, comments, or pull request descriptions.

## Code conventions

- All code, identifiers, comments, and documentation are in English.
- No emoji anywhere in code, comments, documentation, commit messages, or CLI/TUI output.
  Use plain ASCII status indicators such as `[OK]` and `[FAIL]`.
- Every exported identifier has a GoDoc comment.
- Every source file starts with `// SPDX-License-Identifier: Apache-2.0`.
- Errors are wrapped with `fmt.Errorf("...: %w", err)` and compared with `errors.Is` /
  `errors.As`. Library code does not `panic`.
- `context.Context` is propagated through any call that can block, be canceled, or time
  out.
- Only the dependencies already present in `go.mod` are used, plus the standard library.
  Proposing a new dependency requires justification in the pull request description.
- Security-sensitive code (command execution, SSH, secrets, approval gates, rollback)
  should fail closed, never silently skip a check, and never log a secret value.

## Reporting bugs and requesting features

Use the issue templates. For suspected security vulnerabilities, follow
[`SECURITY.md`](SECURITY.md) instead of opening a public issue.

## Code of conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md).
