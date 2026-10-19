# Security Policy

OpsRun executes commands on remote infrastructure during incident response. Security
issues are taken seriously and are handled privately until a fix is available.

## Reporting a vulnerability

Do not open a public GitHub issue for a suspected security vulnerability. Instead, use
GitHub's private vulnerability reporting for this repository:
`https://github.com/Leo-Galli/opsrun/security/advisories/new`.

Include, as applicable:

- A description of the vulnerability and its potential impact.
- Steps to reproduce, including a minimal runbook or configuration if relevant.
- The OpsRun version (`opsrun version`) and environment where you found the issue.

You should expect an initial response within 5 business days. We will coordinate with
you on a disclosure timeline once the report is confirmed.

## Supported versions

Security fixes are released for the latest minor version on the `main` branch. Older
tags do not receive backported fixes.

## Scope

In scope: the `opsrun` CLI and its core packages (`internal/...`), the runbook parser and
validator, the SSH transport, the execution engine, the audit log, and the official
release artifacts and container images.

Out of scope: vulnerabilities in third-party dependencies should be reported upstream;
if a dependency vulnerability is directly exploitable through OpsRun, please report it to
us as well so we can track the upgrade.
