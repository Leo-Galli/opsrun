<div align="center">

<a href="https://github.com/Leo-Galli/opsrun"><img src="https://capsule-render.vercel.app/api?type=waving&height=180&color=0:0b1220,55:0f2a4a,100:0e7490&text=OpsRun&fontColor=e8eef5&fontSize=64&fontAlignY=45&animation=twinkling" alt="OpsRun" width="100%" /></a>

<img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=500&size=16&color=38BDF8&center=true&vCenter=true&width=680&height=34&duration=2800&pause=1200&lines=Markdown+runbooks+to+safe+SSH+workflows;DAG+%7C+rollback+%7C+approvals+%7C+audit;Nine+ready+incident+runbooks;Go+CLI+%2B+Bubble+Tea+TUI" alt="OpsRun tagline" />

<p>
  <a href="https://github.com/Leo-Galli/opsrun"><img src="https://skillicons.dev/icons?i=github,go,docker,md,bash,linux,astro&theme=dark&perline=7" alt="GitHub, Go, Docker, Markdown, Bash, Linux, Astro" /></a>
</p>

<p>
  <a href="https://github.com/Leo-Galli/opsrun/actions/workflows/ci.yml"><img src="https://github.com/Leo-Galli/opsrun/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/Leo-Galli/opsrun/actions/workflows/lint.yml"><img src="https://github.com/Leo-Galli/opsrun/actions/workflows/lint.yml/badge.svg" alt="Lint" /></a>
  <a href="https://github.com/Leo-Galli/opsrun/actions/workflows/security.yml"><img src="https://github.com/Leo-Galli/opsrun/actions/workflows/security.yml/badge.svg" alt="Security" /></a>
  <a href="https://pkg.go.dev/github.com/Leo-Galli/opsrun"><img src="https://pkg.go.dev/badge/github.com/Leo-Galli/opsrun.svg" alt="Go Reference" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License" /></a>
  <img src="https://img.shields.io/badge/status-v0.1.0-0e7490.svg" alt="Status v0.1.0" />
</p>

</div>

<br/>

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=WHAT+THIS+IS" alt="What this is" /></p>

OpsRun turns Markdown runbooks into safe, concurrent incident-response workflows over SSH.

Under pressure, wiki copy-paste fails: no dependency order, no rollback, no approvals, no
tamper-evident record. OpsRun treats the runbook as executable, auditable code.

| Capability | What you get |
|---|---|
| DAG execution | Independent steps run concurrently; dependents wait |
| Rollback | Failed runs reverse successful destructive steps |
| Approvals | Operators see the exact command and hosts before exec |
| Audit | Append-only, hash-chained log for post-mortems |
| Rollout | `all`, `rolling`, or `canary` host strategies |
| Resume | Checkpoint and continue after interrupt or failure |

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=PRINCIPLES" alt="Principles" /></p>

- Validate before execute; never invent a successful run
- Destructive steps declare rollback; resume from checkpoints
- Approvals show the interpolated command and target hosts first
- SSH host-key verification is mandatory by default
- Secrets stay out of logs and the audit trail
- The audit log is append-only and hash-chained

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=INSTALL" alt="Install" /></p>

<details>
<summary><strong>Prebuilt binary</strong></summary>

```bash
curl -LO https://github.com/Leo-Galli/opsrun/releases/latest/download/opsrun_linux_amd64.tar.gz
curl -LO https://github.com/Leo-Galli/opsrun/releases/latest/download/checksums.txt
sha256sum --ignore-missing -c checksums.txt
tar -xzf opsrun_linux_amd64.tar.gz
sudo install -m 0755 opsrun /usr/local/bin/opsrun
```

</details>

<details>
<summary><strong>go install</strong></summary>

```bash
go install github.com/Leo-Galli/opsrun/cmd/opsrun@latest
```

</details>

<details>
<summary><strong>Docker</strong></summary>

```bash
docker run --rm -v "$PWD":/work -w /work \
  -v "$HOME/.ssh":/root/.ssh:ro \
  ghcr.io/leo-galli/opsrun:latest run runbook.md --dry-run
```

</details>

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=QUICK+START" alt="Quick start" /></p>

Try any bundled runbook with zero SSH access:

```bash
opsrun validate examples/runbooks/full-disk.md
opsrun plan examples/runbooks/lb-node-drain.md --var lb_api_token=dummy
```

Run for real (approvals + automatic rollback):

```bash
opsrun run examples/runbooks/full-disk.md \
  --config opsrun.yaml \
  --vars-file prod.vars.yaml \
  --interactive
```

Rolling rollout, resume, and audit:

```bash
opsrun run examples/runbooks/rolling-deploy.md --config opsrun.yaml \
  --var release_tag=v1.4.2 \
  --strategy rolling --batch-size 5 --max-failure-percent 10

opsrun resume <run-id> --checkpoint-dir .opsrun/checkpoints
opsrun audit verify --log ./audit/opsrun-audit.jsonl
opsrun audit export --log ./audit/opsrun-audit.jsonl --run-id <run-id> > postmortem.md
```

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=520&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=READY+RUNBOOKS" alt="Ready runbooks" /></p>

Nine production-shaped runbooks in [`examples/runbooks/`](examples/runbooks/). Each is
validated and plan-tested in CI. Copy, adapt the vars, point at your inventory.

| Runbook | Severity | Service | What it does | Try now |
|---|---|---|---|---|
| [`full-disk.md`](examples/runbooks/full-disk.md) | high | web-frontend | Check disk, clear cache, restart with rollback | `opsrun validate examples/runbooks/full-disk.md` |
| [`service-restart.md`](examples/runbooks/service-restart.md) | medium | payments-api | Pre-check unhealthy service, restart, healthz, rollback | `opsrun plan examples/runbooks/service-restart.md` |
| [`db-failover.md`](examples/runbooks/db-failover.md) | critical | orders-db | Lag-checked replica promote + app repoint (`approval: all`) | `opsrun validate examples/runbooks/db-failover.md` |
| [`cert-renewal.md`](examples/runbooks/cert-renewal.md) | high | edge-proxy | Expiry check, renew TLS, backup/restore rollback | `opsrun plan examples/runbooks/cert-renewal.md --var domain=example.com` |
| [`oom-mitigation.md`](examples/runbooks/oom-mitigation.md) | critical | api-worker | Confirm memory pressure, restart worker safely | `opsrun validate examples/runbooks/oom-mitigation.md` |
| [`log-cleanup.md`](examples/runbooks/log-cleanup.md) | medium | api-gateway | Compress/delete old logs under retention policy | `opsrun plan examples/runbooks/log-cleanup.md` |
| [`lb-node-drain.md`](examples/runbooks/lb-node-drain.md) | medium | web-frontend | Drain via LB API (`secret: true` token), maintain, restore | `opsrun plan examples/runbooks/lb-node-drain.md --var lb_api_token=dummy` |
| [`rolling-deploy.md`](examples/runbooks/rolling-deploy.md) | medium | web-frontend | Deploy release with rolling/canary host strategy | `opsrun plan examples/runbooks/rolling-deploy.md --var release_tag=v1.4.2` |
| [`runaway-process.md`](examples/runbooks/runaway-process.md) | high | batch-worker | Identify runaway PID, SIGTERM then SIGKILL | `opsrun plan examples/runbooks/runaway-process.md --var process_name=worker` |

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=RUNBOOK+FORMAT" alt="Runbook format" /></p>

Plain Markdown + YAML front matter. Steps are `## Step: <id> - <title>` with fenced
blocks:

| Info string | Role |
|---|---|
| `` ```bash opsrun:run `` | Action to execute |
| `` ```bash opsrun:check `` | Post-condition |
| `` ```bash opsrun:rollback `` | Compensating action |

Directives next to the heading: `depends_on`, `confirm`, `retries`, `continue_on_error`,
`read_only`. Variables use `{{ .Vars.name }}` and are shell-escaped on interpolate.

Full grammar: [`docs/runbook-spec.md`](docs/runbook-spec.md).

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=LAYOUT" alt="Layout" /></p>

| Path | Role |
|---|---|
| `cmd/opsrun` | Cobra CLI + Bubble Tea TUI |
| `internal/` | Parser, DAG engine, SSH transport, audit, approval |
| `examples/runbooks/` | Nine ready incident runbooks |
| `website/` | Product site + docs (Astro) → [opsrun.vercel.app](https://opsrun.vercel.app) |
| `docs/` | Spec, architecture, issue/PR guidelines |
| `.github/` | CI, issue forms, PR template |
| `opsrun.yaml` | Sample inventory and runtime config |

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=STACK" alt="Stack" /></p>

| Piece | Tech |
|---|---|
| Language | Go (toolchain pinned in `go.mod`) |
| CLI / TUI | Cobra, Bubble Tea, Lip Gloss |
| Runbooks | Markdown (goldmark AST) + YAML front matter |
| Remote exec | SSH via `golang.org/x/crypto/ssh` |
| Website | Astro + Tailwind (`website/`, deploy Root Directory `website`) |
| Release | GoReleaser, SBOM, multi-arch image on `ghcr.io` |

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=DOCS" alt="Docs" /></p>

| Doc | Role |
|---|---|
| [`docs/runbook-spec.md`](docs/runbook-spec.md) | Grammar and validation rules |
| [`docs/architecture.md`](docs/architecture.md) | Components and execution flow |
| [`docs/pull-request-guidelines.md`](docs/pull-request-guidelines.md) | PR size, commits, review bar |
| [`docs/issue-guidelines.md`](docs/issue-guidelines.md) | Bug / feature filing |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Local workflow and conventions |
| [`SECURITY.md`](SECURITY.md) | Vulnerability reporting |
| [`CHANGELOG.md`](CHANGELOG.md) | Release history |

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=DEVELOP" alt="Develop" /></p>

```bash
make build   # ./bin/opsrun
make test    # go test -race ./...
make lint    # golangci-lint run
make vuln    # govulncheck ./...
```

Architecture deep dive: [`docs/architecture.md`](docs/architecture.md). Security model and
host-key / secret rules: [`SECURITY.md`](SECURITY.md).

<p align="center"><sub><i>A good incident run is like a well-refereed match: when everything works, nobody invents the score.</i></sub></p>

## License

[Apache License 2.0](LICENSE) · [NOTICE](NOTICE)

<img src="https://capsule-render.vercel.app/api?type=waving&section=footer&height=110&color=0:0b1220,55:0f2a4a,100:0e7490" alt="" width="100%" />
