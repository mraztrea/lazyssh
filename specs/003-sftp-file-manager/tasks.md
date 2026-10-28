---

description: "Task list for SFTP File Manager"
---

# Tasks: SFTP File Manager

**Input**: Design documents from `/specs/003-sftp-file-manager/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/sftp-api.md, quickstart.md

**Tests**: Required. Constitution II and the source plan mandate TDD: write the test, see it fail,
implement, see it pass, commit. The Task 12 screen (tview wiring) is verified manually
(quickstart.md); its pane helpers have unit tests.

**Organization**: Phases map 1:1 onto source plan (`docs/2026-10-28-sync-keys-sftp-plan.md`)
Tasks 10–12. Each phase ends in exactly one commit with the source plan's message, ending with the
session's `Co-Authored-By` line. Story labels show which spec user stories each task serves.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 browse/connect, US2 upload/download, US3 mkdir/rename/delete

## Phase 1: Setup

Folded into Phase 2 (T003): the only setup is `go get github.com/pkg/sftp@v1.13.12`, which the
source plan lists under Task 10.

---

## Phase 2: Source Task 10 "SFTP session over system ssh" (US1)

**Goal**: `ports.SFTPService.Connect(alias)` returns a working `*sftp.Client` over `ssh -s -- <alias>
sftp`; `Close` kills the process; an early ssh exit is an error, not a hang.

**Independent Test**: `go test ./internal/core/services/ -run 'TestConnect' -v`

- [X] T001 [US1] Write failing tests in new file internal/core/services/sftp_service_test.go (Apache header): `helperCmd(mode string) func(alias string) *exec.Cmd` re-running the test binary with `-test.run=TestHelperProcess` and env `GO_WANT_SFTP_HELPER=<mode>`; `TestHelperProcess` (returns unless the env is set; `fail` → write `ssh: connect to host any port 22: Connection refused` to stderr and `os.Exit(255)`; `1` → `sftp.NewServer(stdio rwc)` + `Serve()` then `os.Exit(0)`); `TestConnect_ListsDirectoryAndCloses` (ReadDir of a `t.TempDir()` with one file → 1 entry; `Close()` nil); `TestConnect_ProcessExitReturnsError` (Connect in goroutine, `done` channel vs `time.After(5*time.Second)`; error non-nil and contains `Connection refused`)
- [X] T002 [US1] Run `go test ./internal/core/services/ -run 'TestConnect' -v`; confirm FAIL
- [X] T003 [US1] Run `go get github.com/pkg/sftp@v1.13.12`; verify go.mod still has `golang.org/x/crypto v0.58.0` (no downgrade)
- [X] T004 [P] [US1] Append to internal/core/ports/services.go: `SFTPSession{ Client() *sftp.Client; Close() error }`, `SFTPService{ Connect(alias string) (SFTPSession, error) }`
- [X] T005 [US1] Create internal/core/services/sftp_service.go (Apache header): `NewSFTPService(logger *zap.SugaredLogger) ports.SFTPService` → `newSFTPServiceWithCmd(logger, func(alias string) *exec.Cmd { return exec.Command("ssh", "-s", "--", alias, "sftp") })` (corrected from the source plan, see T021a); `Connect` = `StdinPipe`, `StdoutPipe`, `cmd.Stderr` = tail buffer (last 4 KiB), `Start`, `sftp.NewClientPipe(stdout, stdin)`; on error `Process.Kill()` + `Wait()`, log alias + stderr tail, return `fmt.Errorf("sftp %s: %w: %s", alias, err, lastLine)` (no suffix when empty); session `Close` (`sync.Once`) = `client.Close()`, `stdin.Close()`, `Process.Kill()`, `Wait()` (wait error ignored), logs the stderr tail (if any) at debug level, returns client close error; comment `// ponytail: subprocess transport. Upgrade to golang.org/x/crypto/ssh only if an in-process client is ever required.`
- [X] T006 [US1] Wire: cmd/main.go `sftpService := services.NewSFTPService(log)`; internal/adapters/ui/tui.go `NewTUI(logger, ss, sync, sftp ports.SFTPService, version, commit)` storing `sftpService`; update every `NewTUI` call
- [X] T007 [US1] Run `make test && make build`; confirm PASS
- [X] T008 [US1] Commit `feat: open sftp sessions through the system ssh client` (go.mod go.sum cmd internal)

---

## Phase 3: Source Task 11 "Transfer operations" (US2, US3)

**Goal**: Upload/download files and directory trees with progress and preserved file modes;
recursive remote delete.

**Independent Test**: `go test ./internal/core/services/ -run 'Upload|Download|RemoveRemote' -v`

- [X] T009 [US2] [US3] Write failing tests in new file internal/core/services/sftp_transfer_test.go (Apache header): helper `pipeClient(t)` (`net.Pipe()`, `sftp.NewServer(serverConn)` + `Serve` in a goroutine, `sftp.NewClientPipe(clientConn, clientConn)`, cleanup closes both); `TestUploadPath_FileWithProgress` (3 MiB random file mode 0640 → remote dir; content equal; last progress `Done==Total==3<<20`, `FileCount==1`; dest mode `0640`); `TestUploadPath_DirectoryRecursive` (`a/x.txt`, `a/b/c.txt` → `dest/a/x.txt`, `dest/a/b/c.txt`; progress `FileCount==2`); `TestDownloadPath_FileAndDirectory` (file and tree copied into a local dir, contents equal); `TestRemoveRemote_Recursive` (tree `r/s/t.txt` + `r/u.txt` removed; `Lstat(r)` not-exist)
- [X] T010 [US2] [US3] Run `go test ./internal/core/services/ -run 'Upload|Download|RemoveRemote' -v`; confirm FAIL
- [X] T011 [US2] [US3] Create internal/core/services/sftp_transfer.go (Apache header): `TransferProgress{ File string; Done, Total int64; FileIndex, FileCount int }`; `UploadPath` (`filepath.WalkDir` plan phase: dirs + regular files, symlinks `os.Stat`ed, non-regular skipped; then `c.MkdirAll` per dir and copy each file); `DownloadPath` (`c.RealPath` root, `c.Walk` plan phase, symlink entries `c.Stat`ed; `os.MkdirAll(…, 0o750)`; copy); copy = open dst truncating, `io.CopyBuffer(counting writer, struct{io.Reader}{src}, 1 MiB)`, close, `Chmod(mode.Perm())` best effort; `RemoveRemote` (`c.Lstat`; dir → `c.ReadDir` recurse + `c.RemoveDirectory`; else `c.Remove`); log nothing about contents
- [X] T012 [US2] [US3] Run `make test`; confirm PASS
- [X] T013 [US2] [US3] Commit `feat: add sftp upload, download and recursive remove` (internal/core/services/sftp_transfer.go internal/core/services/sftp_transfer_test.go)

---

## Phase 4: Source Task 12 "File manager screen" (US1, US2, US3)

**Goal**: Key `m` opens the dual-pane file manager implementing spec FR-006..FR-016.

**Independent Test**: `go test ./internal/adapters/ui/ -run 'SortEntries|FormatSize|FilePane' -v`
plus quickstart.md manual steps.

- [X] T014 [US1] Write failing tests in new file internal/adapters/ui/sftp_pane_test.go (Apache header): `TestSortEntries_DirsFirstCaseInsensitive` (`[b.txt, A/, a.txt, B/]` → `[A/, B/, a.txt, b.txt]`); `TestFormatSize` (`0→"0"`, `512→"512"`, `1234→"1.2K"`, `3_500_000→"3.3M"`, `1_200_000_000→"1.1G"`); `TestFilePane_LoadRendersParentRow` (fake list returns 2 entries; `GetRowCount()==3+1` header; row 1 cell 0 text `../` and `Selected()` on row 1 gives Name `..`; `cwd` set; failing list keeps old rows and cwd)
- [X] T015 [US1] Run `go test ./internal/adapters/ui/ -run 'SortEntries|FormatSize|FilePane' -v`; confirm FAIL
- [X] T016 [US1] Create internal/adapters/ui/sftp_pane.go (Apache header): `fileEntry{ Name string; IsDir bool; Size int64; ModTime time.Time }`; `sortEntries` (dirs first, then `strings.ToLower` name); `formatSize` (1024-based, `%.1f` + `K/M/G`, bytes as integer below 1024); `filePane{ *tview.Table; title, cwd string; list func(string) ([]fileEntry, error); join func(...string) string }`; `newFilePane(title, list)` (bordered table, fixed header row `Name/Size/Modified`, `join = filepath.Join`); `Load(path)` (list → on error return it unchanged; sort; rows header, `../`, `name/` for dirs, size blank for dirs, `ModTime.Format("01-02")`; set cwd; title `" <title>: <cwd> "`; select row 1); `Selected()`; `ParentDir()` = `join(cwd, "..")`; `localList(path)` (os.ReadDir + Info, symlink → os.Stat) 
- [X] T017 [US1] Run the pane tests; confirm PASS
- [X] T018 [US1] [US2] [US3] Create internal/adapters/ui/sftp_view.go (Apache header): `handleFileManager()` (selected server; `app.Suspend(func(){ sess, err = t.sftpService.Connect(alias) })`; err → `showStatusTempColor("SFTP failed: "+err.Error(), "#FF6B6B")`; else `showFileManager(alias, sess)`); `showFileManager`: local pane start `os.UserHomeDir()` (fallback `/`), remote pane `join = path.Join`, list via `client.ReadDir` (symlinks `client.Stat`ed), start `client.Getwd()`, title `Remote` + ` (alias)` suffix; `Flex` columns + 1-line status `TextView` showing key hints; input capture per spec FR-009 (Tab, Enter, Backspace/h, j/k, u, d, n, r, D, R, g, q/Esc); `..` ignored by u/d/r/D; `Enter` on a file does nothing; `u` always uses the local selection and `d` the remote selection regardless of the active pane (FR-010); empty prompt input or a `/` in a mkdir/rename name does nothing; prompts (one-field Form, Esc cancels) `New directory:`, `Rename to:`, `Go to path:`; modals `Overwrite <name>?` and `Delete <name>?`; transfers: `atomic.Bool` guard (`A transfer is already running`), destination `Stat` → overwrite modal, goroutine calling `services.UploadPath/DownloadPath`, progress throttled ~100 ms via `QueueUpdateDraw` (`Uploading <file>  [n/total files  ]NN%  X.X MB/s`), finish → reload both panes + green `Uploaded <name>`/`Downloaded <name>` or red `Upload failed: <err>`; local ops `os.Mkdir(…, 0o750)`, `os.Rename`, `os.RemoveAll`; remote ops `client.Mkdir`, `client.Rename`, `services.RemoveRemote`; `q`/`Esc` → `sess.Close()` → `returnToMain()`; log paths/sizes/errors only
- [X] T019 [US1] internal/adapters/ui/handlers.go: `case 'm': t.handleFileManager(); return nil`; internal/adapters/ui/status_bar.go: append `• [white]m[-] Files`
- [X] T020 [P] [US1] README.md: Features entry for the SFTP file manager and `m` row in the key-binding table
- [X] T021 [US1] [US2] [US3] Manual verification per quickstart.md in tmux (open `m`, navigate both panes, `u`, `d`, `n`, `r`, `D` with confirm, `q`, `pgrep -f 'ssh -s sftp'` empty)
- [X] T021a [US1] Fix found by T021: OpenSSH `-s` takes no argument, so `ssh -s sftp <alias>` connected to host `sftp`. Failing test `TestNewSFTPService_RequestsSubsystemAfterAlias` (argv `ssh -s -- web-1 sftp`) in internal/core/services/sftp_service_test.go → fix in internal/core/services/sftp_service.go → commit `fix: put the alias before the sftp subsystem name in ssh -s`. Also fixed in internal/adapters/ui/sftp_view.go before the Task 12 commit: `show()` keeps the active pane (SetRoot refocused the local pane after every prompt/modal); added `TestProgressText`
- [X] T022 [US1] Run `make build && make test`; confirm PASS
- [X] T023 [US1] Commit `feat: add sftp dual-pane file manager` (internal/adapters/ui README.md)

---

## Phase 5: Polish

- [X] T024 Commit spec-kit artifacts `docs: add speckit artifacts for sftp file manager` (specs/003-sftp-file-manager .specify/feature.json); fast-forward `feat/sync-keys-sftp` to `003-sftp-file-manager`

## Dependencies

- Phase 2 (Task 10) → Phase 3 (Task 11, needs pkg/sftp) → Phase 4 (Task 12, needs both).
- Within phases: test → fail → implement → pass → commit, strictly in order. T004 can be written
  alongside T001.

## Parallel Example

```text
T004 (ports) while T001 (tests) are written; T020 (README) while T018 (screen) is written.
```

## Implementation Strategy

MVP = Phases 2 + 4 browse-only (US1). Ship order follows the source plan: 10 → 11 → 12, one commit
each, `make test` green after each.
