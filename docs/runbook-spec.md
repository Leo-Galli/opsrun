# Runbook format specification

An OpsRun runbook is a Markdown file with a YAML front matter block and a sequence of
level-2 headings, one per step. The parser (`internal/parser`) walks the Markdown AST with
goldmark; it never matches on raw text with regular expressions for structural elements,
only for the small, fixed grammar described below (step headings and code block info
strings).

## Front matter

Every runbook must start with a YAML front matter block delimited by `---` lines:

```yaml
---
name: full-disk-remediation       # required
version: "1.0"
service: web-frontend
severity: high                    # low | medium | high | critical
author: sre-team
targets:
  - group: web                    # one of group or host per entry
  - host: db-primary
vars:
  cache_dir:
    type: string                  # string | int | bool | duration
    default: "/var/cache/app"
    required: false
    validation: "^/[a-zA-Z0-9/_-]+$"   # optional regular expression
  api_token:
    type: string
    required: true
    secret: true                  # never written to logs, events, or the audit trail
approval:
  mode: per_step                  # none | per_step | all
  timeout: 5m
---
```

Unknown front matter fields are a parse error (the YAML decoder runs with strict,
known-fields-only decoding), so typos are caught immediately rather than silently
ignored. `name` is the only required field.

## Steps

Each step is a level-2 ATX heading with this exact shape:

```markdown
## Step: <id> - <title>
```

`<id>` matches `[A-Za-z0-9_-]+` and must be unique across the runbook (enforced by the
validator, not the parser, so a runbook with a duplicate id still parses and can be
inspected with `opsrun validate`). `<title>` is free text.

### Directive block (optional)

If the paragraph immediately following a step heading parses as a YAML mapping whose
keys are all recognized directive names, it is consumed as that step's directives and
does not appear in the step's description. Recognized keys:

| Key                | Type       | Meaning                                             |
|--------------------|------------|------------------------------------------------------|
| `depends_on`       | `[]string` | ids of steps that must succeed before this one runs |
| `confirm`          | `bool`     | require explicit human approval before this step    |
| `retries`          | `int`      | number of times to retry the run action on failure  |
| `continue_on_error`| `bool`     | keep the run going if this step fails                |
| `mode`             | `string`   | `serial` or `parallel` execution across target hosts |
| `read_only`        | `bool`     | marks a diagnostic step with no side effects         |
| `approval_mode`    | `string`   | overrides the runbook-wide approval mode for this step |
| `approval_timeout` | `string`   | duration string, overrides the approval timeout      |

Example:

```markdown
## Step: clear-cache - Clear application cache
depends_on: [check-disk]
confirm: true

Clears the on-disk cache directory before restarting the service.
```

Any paragraph that does not parse as a mapping of only recognized directive keys is
treated as prose and appended to the step's description instead, so ordinary explanatory
text is never mistaken for directives.

### Action code blocks

A step's behavior is defined by fenced code blocks whose info string starts with the
language, then an `opsrun:<kind>` marker, then zero or more `key=value` attributes:

```markdown
​```bash opsrun:run target=web timeout=30s
systemctl restart web-frontend
​```

​```bash opsrun:check expect_exit=0
systemctl is-active web-frontend
​```

​```bash opsrun:rollback
systemctl restart web-frontend
​```
```

Supported kinds and attributes:

- `opsrun:run` — the step's main action. Attributes: `target` (inventory host or group;
  if omitted, the runbook's top-level `targets` apply), `timeout` (Go duration string).
  At most one per step.
- `opsrun:check` — a post-condition evaluated after the run action succeeds. Attributes:
  `expect_exit` (integer, default `0`). A step may have any number of checks.
- `opsrun:rollback` — the compensating action run, in reverse step order, if a later step
  in the run fails. Attributes: `target`, `timeout`. At most one per step.

A fenced code block whose info string does not contain an `opsrun:` marker is left alone
and is not treated as an action; it is useful for showing illustrative, non-executed
snippets inside a step's prose.

### Variables

Commands reference declared variables with `{{ .Vars.name }}`. At validation time, every
reference must resolve to a name declared in the front matter `vars` map; an undeclared
reference is a validation error, never a silent empty substitution. At execution time,
values are shell-escaped (wrapped in single quotes, with embedded single quotes escaped)
before interpolation, so a variable value can never break out of its argument position
regardless of its contents.

A variable declared with `secret: true` is interpolated with its real value only in the
command actually sent to a host. Everywhere else it might otherwise appear — approval
prompts, the TUI, emitted events, the audit log — it is replaced with `***REDACTED***`.
Use `secret: true` for anything that should never be written to disk in plain text:
API tokens, passwords, connection strings with embedded credentials, and the like.

## Validation

`internal/parser.MarkdownValidator` checks a parsed runbook beyond what the grammar
above can express on its own:

- Duplicate step ids.
- Dependencies (`depends_on`) referring to a step id that does not exist.
- Dependency cycles.
- Variable references that are not declared in the front matter.
- Commands matching a denylist of dangerous patterns (for example `rm -rf /`), reported
  as errors. The built-in denylist can be extended with additional regular expressions.
- Steps whose run action matches a broader set of destructive patterns (`rm -rf`,
  `DROP TABLE`, `systemctl stop`, `shutdown`, `reboot`, `kill -9`, and similar) but have
  no `opsrun:rollback` block, reported as warnings.

`opsrun validate <runbook.md>` runs the parser and the validator and prints every
finding with its step id and source position.
