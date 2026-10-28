# lazyssh Constitution

## Core Principles

### I. Hexagonal Layering

Code is layered `domain → ports → services → adapters`, under `internal/core/{domain,ports,services}`
and `internal/adapters/{data,ui}`.

- `domain` MUST NOT import any other project package.
- `services` MUST depend on `ports` interfaces, never on concrete adapters.
- The UI adapter (`internal/adapters/ui`) MUST NOT import `internal/adapters/data`; it talks to
  services through `ports`.
- Wiring of concrete adapters happens only in `cmd/`.

Rationale: keeps storage and presentation swappable and logic testable without a terminal.

### II. Test-First (NON-NEGOTIABLE)

- Every non-trivial function (a branch, a loop, a parser, a file or security path) MUST have a test
  written before the implementation, seen failing, then made to pass.
- `go test -race ./...` MUST be green before every commit.
- Tests that touch the filesystem MUST use `t.TempDir()`; tests MUST NOT read or write the real
  `~/.ssh` or `~/.lazyssh`.

Rationale: the tool edits users' SSH config; regressions there cost real access.

### III. Minimal Dependencies

- The Go standard library is the first choice.
- New third-party modules are limited to those named in an approved spec:
  `github.com/pkg/sftp` and `golang.org/x/crypto`. Any other addition requires a constitution
  amendment.

Rationale: a small dependency graph keeps the single static binary auditable.

### IV. Security of Secrets

- Files written under `~/.lazyssh` MUST be mode `0600`; directories MUST be mode `0700`.
- Private key text MUST NEVER appear in logs, the status bar, error messages, or the details pane.
  Logs identify servers by alias only, never by dumping a `domain.Server` value.
- Secrets MUST NOT be committed to the repository, including test fixtures; tests generate keys at
  runtime.

Rationale: lazyssh stores embedded private keys and sync tokens; leaking them is unrecoverable.

### V. Simplicity (YAGNI)

- No interface, factory, or config knob with a single implementation or a single value.
- The shortest diff that satisfies the spec wins; speculative features are out of scope until a
  spec asks for them.
- Deliberate shortcuts with a known ceiling are marked in code with a `ponytail:` comment naming
  the ceiling and the upgrade path.

Rationale: a TUI maintained by a small team cannot afford abstraction debt.

## Technical Constraints

- Language: Go 1.24+; TUI: `github.com/rivo/tview` / `tcell`.
- SSH config is the source of truth for host settings; lazyssh-only data lives in
  `~/.lazyssh/metadata.json`.
- Every new `.go` file carries the Apache 2.0 license header used across the repository.

## Development Workflow

- Quality gates: `make build` (runs gofumpt, vet, golangci-lint) and `make test`
  (`go test -race ./...`) MUST pass before a change is merged. Lint findings are fixed, not
  suppressed by disabling linters.
- Commit messages use the prefixes `feat:`, `fix:`, `improve:`, `test:`, `docs:`.
- Feature work follows the spec-kit flow: spec → plan → tasks → implement, with one commit per
  task.

## Governance

- This constitution supersedes conflicting practices in the repository. Reviews MUST check
  changes against Principles I–V.
- Amendments are made by pull request that edits this file, states the reason, and bumps the
  version: MAJOR for removing or redefining a principle, MINOR for adding a principle or section or
  materially expanding guidance, PATCH for wording fixes.
- Any justified violation MUST be recorded in the feature plan's Complexity Tracking table.

**Version**: 1.0.0 | **Ratified**: 2026-10-28 | **Last Amended**: 2026-10-28
