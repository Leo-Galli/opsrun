<div align="center">

<a href="https://github.com/Leo-Galli/opsrun"><img src="https://capsule-render.vercel.app/api?type=waving&height=180&color=0:0b1220,55:0f2a4a,100:0e7490&text=OpsRun&fontColor=e8eef5&fontSize=64&fontAlignY=45&animation=twinkling" alt="OpsRun" width="100%" /></a>

<img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=500&size=16&color=38BDF8&center=true&vCenter=true&width=640&height=34&duration=2800&pause=1200&lines=Markdown+runbooks+%E2%86%92+safe+SSH+workflows;DAG+executor+%C2%B7+rollback+%C2%B7+approvals;Hash-chained+audit+log;Go+CLI+%2B+Bubble+Tea+TUI" alt="OpsRun tagline" />

<p>
  <a href="https://github.com/Leo-Galli/opsrun"><img src="https://skillicons.dev/icons?i=github&theme=dark" width="44" height="44" alt="GitHub" /></a>&nbsp;
  <a href="https://go.dev"><img src="https://skillicons.dev/icons?i=go&theme=dark" width="44" height="44" alt="Go" /></a>&nbsp;
  <a href="https://www.docker.com"><img src="https://skillicons.dev/icons?i=docker&theme=dark" width="44" height="44" alt="Docker" /></a>&nbsp;
  <a href="https://yaml.org"><img src="https://skillicons.dev/icons?i=yaml&theme=dark" width="44" height="44" alt="YAML" /></a>
</p>

<p>
  <a href="https://github.com/Leo-Galli/opsrun/actions/workflows/ci.yml"><img src="https://github.com/Leo-Galli/opsrun/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/Leo-Galli/opsrun/actions/workflows/lint.yml"><img src="https://github.com/Leo-Galli/opsrun/actions/workflows/lint.yml/badge.svg" alt="Lint" /></a>
  <a href="https://github.com/Leo-Galli/opsrun/actions/workflows/security.yml"><img src="https://github.com/Leo-Galli/opsrun/actions/workflows/security.yml/badge.svg" alt="Security" /></a>
  <a href="https://pkg.go.dev/github.com/Leo-Galli/opsrun"><img src="https://pkg.go.dev/badge/github.com/Leo-Galli/opsrun.svg" alt="Go Reference" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License" /></a>
</p>

</div>

<br/>

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=WHAT+THIS+REPO+IS" alt="What this repo is" /></p>

OpsRun turns Markdown runbooks into safe, concurrent incident-response workflows over SSH.
DAG execution, automatic rollback, human approval gates, and a hash-chained audit log --
measured behavior, not improvisation under pressure.

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=PRINCIPLES" alt="Principles" /></p>

- Validate before execute; never invent a successful run
- Destructive steps declare rollback; resume from checkpoints
- Approvals show the exact command and target hosts first
- SSH host-key verification is mandatory by default
- Secrets are redacted from logs and the audit trail
- The audit log is append-only and hash-chained

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=LAYOUT" alt="Layout" /></p>

| Path | Role |
|---|---|
| `cmd/opsrun` | Cobra CLI and Bubble Tea TUI |
| `internal/` | Parser, DAG engine, SSH transport, audit, approval |
| `examples/runbooks/` | Ready-to-adapt incident runbooks |
| `docs/` | Architecture, runbook grammar, contribution guides |
| `.github/` | CI, issue forms, PR template |
| `opsrun.yaml` | Sample inventory and runtime configuration |

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=STACK" alt="Stack" /></p>

| Piece | Tech |
|---|---|
| Language | Go (`go.mod` pins the toolchain) |
| CLI | Cobra |
| TUI | Bubble Tea + Lip Gloss |
| Runbooks | Markdown (goldmark AST) + YAML front matter |
| Remote exec | SSH (`golang.org/x/crypto/ssh`) |
| Release | GoReleaser, SBOM, multi-arch Docker (`ghcr.io`) |

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=INSTALL" alt="Install" /></p>

### Prebuilt binaries

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

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=QUICK+START" alt="Quick start" /></p>

Validate and plan without touching hosts:

```bash
opsrun validate examples/runbooks/full-disk.md
opsrun plan examples/runbooks/full-disk.md --vars-file prod.vars.yaml
```

Run with approvals and automatic rollback:

```bash
opsrun run examples/runbooks/full-disk.md \
  --config opsrun.yaml \
  --vars-file prod.vars.yaml \
  --interactive
```

Rolling / canary rollout:

```bash
opsrun run examples/runbooks/service-restart.md --config opsrun.yaml \
  --strategy rolling --batch-size 5 --max-failure-percent 10
```

Resume and audit:

```bash
opsrun resume <run-id> --checkpoint-dir .opsrun/checkpoints
opsrun audit verify --log ./audit/opsrun-audit.jsonl
opsrun audit export --log ./audit/opsrun-audit.jsonl --run-id <run-id> > postmortem.md
```

| Runbook | Scenario |
|---|---|
| `full-disk.md` | Clear cache and restart when disk is full |
| `service-restart.md` | Restart with pre-check, health check, rollback |
| `db-failover.md` | Promote replica and repoint the application |
| `cert-renewal.md` | Renew TLS with backup-and-restore rollback |
| `oom-mitigation.md` | Recover from memory pressure |
| `log-cleanup.md` | Compress and delete old logs |
| `lb-node-drain.md` | Drain a host before maintenance |
| `rolling-deploy.md` | Gradual release with rolling / canary |
| `runaway-process.md` | Stop a runaway process gracefully, then forcefully |

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=DOCS" alt="Docs" /></p>

| Doc | Role |
|---|---|
| [`docs/runbook-spec.md`](docs/runbook-spec.md) | Runbook grammar and validation rules |
| [`docs/architecture.md`](docs/architecture.md) | Components and execution flow |
| [`docs/pull-request-guidelines.md`](docs/pull-request-guidelines.md) | PR size, commits, review bar |
| [`docs/issue-guidelines.md`](docs/issue-guidelines.md) | How to file bugs and features |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Local workflow and coding conventions |
| [`SECURITY.md`](SECURITY.md) | Vulnerability reporting |

<img src="https://capsule-render.vercel.app/api?type=rect&height=3&color=0:0b1220,35:38bdf8,70:c9a227,100:0b1220" width="100%" alt="" />

<br/>

<p align="center"><img src="https://readme-typing-svg.demolab.com?font=IBM+Plex+Mono&weight=700&size=18&color=38BDF8&center=true&vCenter=true&width=420&height=40&duration=1&pause=1000000&letterSpacing=6px&lines=DEVELOP" alt="Develop" /></p>

```bash
make build   # build ./bin/opsrun
make test    # go test -race ./...
make lint    # golangci-lint run
make vuln    # govulncheck ./...
```

Status: **v0.1.0** -- parser, SSH pool, DAG engine with rollback/checkpoints, CLI, TUI,
and hash-chained audit log are implemented and tested. See [`CHANGELOG.md`](CHANGELOG.md).

<p align="center"><sub><i>A good incident run is like a well-refereed match: when everything works, nobody invents the score.</i></sub></p>

## License

[Apache License 2.0](LICENSE) · [NOTICE](NOTICE)

<img src="https://capsule-render.vercel.app/api?type=waving&section=footer&height=110&color=0:0b1220,55:0f2a4a,100:0e7490" alt="" width="100%" />
