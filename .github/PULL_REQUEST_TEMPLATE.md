## Summary

<!-- What changed and why. Link related issues (Fixes #N). -->

## Type of change

- [ ] `feat`: new feature
- [ ] `fix`: bug fix
- [ ] `docs`: documentation only
- [ ] `test`: adding or fixing tests
- [ ] `refactor`: no functional change
- [ ] `chore`: tooling, dependencies, CI
- [ ] `ci`: CI/CD configuration
- [ ] `perf`: performance improvement

## Checklist

- [ ] Change is focused and does not mix unrelated refactors
- [ ] Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)
- [ ] `make build` passes
- [ ] `make vet` passes
- [ ] `make test-race` passes
- [ ] `make lint` passes
- [ ] Tests added or updated where behavior changed (table-driven edge cases preferred)
- [ ] Public interfaces and exported identifiers have GoDoc comments
- [ ] No secrets staged; sensitive values redacted in this description
- [ ] No emoji in commits, code, docs, or this description
- [ ] No AI attribution or `Co-authored-by` for assistants
- [ ] Security-sensitive changes (SSH, secrets, approvals, rollback, audit) fail closed

## Safety impact (if applicable)

<!-- Command execution, approval gates, rollback, host-key verification, audit log, secret handling. Write N/A if none. -->

## Test plan

<!-- Commands run and results. Label automated / manual / unavailable. -->

```bash
make build
make vet
make test-race
make lint
```
