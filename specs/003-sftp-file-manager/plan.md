# Implementation Plan: SFTP File Manager

**Branch**: `003-sftp-file-manager` | **Date**: 2026-10-28 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/003-sftp-file-manager/spec.md`

**Source plan**: `docs/2026-10-28-sync-keys-sftp-plan.md`, Phase 3, Tasks 10–12. Files, signatures
and test names below are copied from it.

## Summary

Open an SFTP session by running `ssh -s -- <alias> sftp` (system OpenSSH, user's config) and speaking
SFTP over its stdin/stdout with `github.com/pkg/sftp`; add stateless upload/download/recursive
remove helpers with progress; and add a dual-pane tview file manager on key `m`.

## Technical Context

**Language/Version**: Go 1.26 toolchain, `go 1.26.0` directive

**Primary Dependencies**: existing `tview`/`tcell`, `zap`; NEW `github.com/pkg/sftp v1.13.12`
(must not downgrade `golang.org/x/crypto v0.58.0`); stdlib `os/exec`, `io`, `path`,
`path/filepath`

**Storage**: none (no lazyssh files written; transfers write user files in user-chosen dirs)

**Testing**: `go test -race ./...` via `make test`; Task 10 helper-process idiom
(`TestHelperProcess` + `GO_WANT_SFTP_HELPER`); Task 11 in-process `sftp.NewServer` over
`net.Pipe()`; Task 12 unit tests for pane helpers, screen verified manually in tmux

**Target Platform**: Linux, macOS (Windows builds; needs OpenSSH `ssh` on PATH)

**Project Type**: TUI desktop CLI tool

**Performance Goals**: transfers stream with a 1 MiB buffer; progress UI updates throttled to
~10/s

**Constraints**: one transfer at a time; UI mutation only via `QueueUpdateDraw`; no file contents
in logs; `Close` kills the subprocess

**Scale/Scope**: 3 tasks, 7 new files (4 source, 3 test), 6 modified files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
|---|---|---|
| I. Hexagonal layering | `ports.SFTPSession`/`ports.SFTPService` in `ports`; `services.NewSFTPService` wired in `cmd/main.go`; UI gets `ports.SFTPService` through `NewTUI`. UI also calls `services.UploadPath/DownloadPath/RemoveRemote` directly, as the source plan specifies (see Complexity Tracking). `services` imports no adapter. | PASS (justified) |
| II. Test-first | Tasks 10, 11 and the Task 12 pane helpers start with the failing tests named in the source plan; tests use `t.TempDir()` only. | PASS |
| III. Minimal deps | Only `github.com/pkg/sftp`, which the constitution names. | PASS |
| IV. Security | No key text involved; ssh stderr tail goes to log/error message only; file contents never logged. | PASS |
| V. Simplicity | Subprocess transport marked `ponytail:`; transfer helpers are plain functions (no interface); no transfer queue. | PASS |

Post-design re-check: PASS; one justified entry in Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/003-sftp-file-manager/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── sftp-api.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
go.mod, go.sum                                   # + github.com/pkg/sftp v1.13.12 (Task 10)
internal/core/ports/services.go                  # + SFTPSession, SFTPService (Task 10)
internal/core/services/
├── sftp_service.go, sftp_service_test.go        # NEW (Task 10)
└── sftp_transfer.go, sftp_transfer_test.go      # NEW (Task 11)
cmd/main.go                                      # NewSFTPService wiring (Task 10)
internal/adapters/ui/
├── tui.go                                       # NewTUI(logger, ss, sync, sftp, version, commit) (Task 10)
├── sftp_pane.go, sftp_pane_test.go              # NEW (Task 12)
├── sftp_view.go                                 # NEW handleFileManager, file manager screen (Task 12)
├── handlers.go                                  # case 'm' (Task 12)
└── status_bar.go                                # "m Files" (Task 12)
README.md                                        # Features + key table (Task 12)
```

**Structure Decision**: Existing hexagonal layout; no new packages.

## Design per task

### Task 10: SFTP session over system ssh

- `ports.SFTPSession{ Client() *sftp.Client; Close() error }`,
  `ports.SFTPService{ Connect(alias string) (SFTPSession, error) }`.
- `services.NewSFTPService(logger *zap.SugaredLogger) ports.SFTPService` →
  `newSFTPServiceWithCmd(logger, func(alias string) *exec.Cmd { return exec.Command("ssh", "-s",
  "--", alias, "sftp") })` (the source plan's `"-s", "sftp", alias` order is invalid for OpenSSH;
  research R1).
- `Connect`: `StdinPipe`, `StdoutPipe`, `cmd.Stderr` = bounded tail buffer (last 4 KiB, research
  R2); `Start`; `sftp.NewClientPipe(stdout, stdin)`; on error `Process.Kill()` + `Wait()`, log the
  stderr tail, return `fmt.Errorf("sftp %s: %w: %s", alias, err, lastLine)` (last line omitted if
  empty). Marked `// ponytail: subprocess transport. Upgrade to golang.org/x/crypto/ssh only if an
  in-process client is ever required.`
- `Close`: `client.Close()`, `stdin.Close()`, `Process.Kill()`, `Wait()` (the kill's wait error is
  expected and ignored); returns the client close error. Idempotent via `sync.Once`.
- Tests: `TestConnect_ListsDirectoryAndCloses`, `TestConnect_ProcessExitReturnsError`
  (+ `TestHelperProcess`).

### Task 11: Transfer operations

- `TransferProgress{ File string; Done, Total int64; FileIndex, FileCount int }` (FileIndex
  1-based).
- `UploadPath(c, localPath, remoteDir, progress)`, `DownloadPath(c, remotePath, localDir,
  progress)`, `RemoveRemote(c, path)`.
- Plan-then-copy: walk the source first (`filepath.WalkDir` / `c.Walk`), collect dirs and regular
  files (symlinks resolved with Stat; non-regular skipped), so `FileCount` is known; then create
  dirs (`c.MkdirAll` / `os.MkdirAll 0o750`) and copy each file with `io.CopyBuffer` and a 1 MiB
  buffer through a counting writer that calls `progress`; `Chmod(mode.Perm())` after, error
  ignored. Download root resolved with `c.RealPath` so a symlinked directory is walked (research
  R5).
- `RemoveRemote`: `Lstat`; dir → `ReadDir` recurse then `RemoveDirectory`; else `Remove`. Never
  follows symlinks.
- Tests: `TestUploadPath_FileWithProgress`, `TestUploadPath_DirectoryRecursive`,
  `TestDownloadPath_FileAndDirectory`, `TestRemoveRemote_Recursive`.

### Task 12: File manager screen

- `fileEntry{ Name string; IsDir bool; Size int64; ModTime time.Time }`, `sortEntries`,
  `formatSize`.
- `filePane{ *tview.Table; title, cwd string; list func(string) ([]fileEntry, error); join
  func(...string) string }`; `newFilePane(title, list)` defaults `join` to `filepath.Join`, the
  remote pane sets `path.Join`. `ParentDir()` = `join(cwd, "..")` (same result as `Dir` for clean
  absolute paths, research R6). `Load(path)` lists, sorts, renders header row + `../` + entries,
  sets `cwd` and the border title `<title>: <cwd>`; on error the pane is unchanged.
  `Selected()` returns the entry under the cursor (`..` included, header excluded).
- `sftp_view.go`: `handleFileManager()` (selected server → `app.Suspend(Connect)` → error: red
  `SFTP failed: <err>` on main status bar; else `showFileManager(alias, sess)`). Screen: `Flex`
  row of both panes + 1-line status `TextView`, set with `app.SetRoot`; input capture implements
  the spec key table; prompts are a one-field `tview.Form`; confirms are `tview.Modal`; transfers
  in a goroutine guarded by an `atomic.Bool`, progress via `QueueUpdateDraw`; `q`/`Esc` →
  `sess.Close()` → `returnToMain()`.
- `handlers.go`: `case 'm': t.handleFileManager()`; `status_bar.go`: `• [white]m[-] Files`;
  README Features + key table.
- Tests: `TestSortEntries_DirsFirstCaseInsensitive`, `TestFormatSize`,
  `TestFilePane_LoadRendersParentRow`; screen per quickstart.md.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| UI imports `internal/core/services` for `UploadPath`, `DownloadPath`, `RemoveRemote`, `TransferProgress` (Principle I says UI talks to services through ports) | Source plan Task 11 defines them as exported `services` functions "exported for the UI"; they are stateless functions over the `*sftp.Client` the port already exposes | A `ports.Transfer` interface would have exactly one implementation (Principle V forbids it) and only re-export the same three functions |
