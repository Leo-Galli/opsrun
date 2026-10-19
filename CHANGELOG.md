# Changelog

All notable changes to this project are documented in this file. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Repository scaffolding: standard Go project layout, core domain and interface
  definitions (`RunbookParser`, `Runbook`/`Step`/`Variable`/`RollbackAction`, `Inventory`,
  `Transport`/`Session`, `Executor`, `DAG`, `ApprovalGate`, `RollbackManager`, `AuditSink`,
  `SecretProvider`), `opsrun.yaml` configuration schema and loader, project tooling
  (Makefile, golangci-lint config, GoReleaser config, Dockerfile), and CI workflows for
  linting, testing, multi-platform builds, and vulnerability scanning.
