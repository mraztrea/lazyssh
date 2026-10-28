# Implementation Plan: Settings File and Encrypted Sync

**Branch**: `002-encrypted-sync` | **Date**: 2026-10-28 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/002-encrypted-sync/spec.md`

**Source plan**: `docs/2026-10-28-sync-keys-sftp-plan.md`, Phase 2, Tasks 4–9. Files, signatures and
test names below are copied from it; Phase 3 changes `ui.NewTUI` again, so the constructor stays a
plain parameter list.

## Summary

Add a machine-local settings file (`~/.lazyssh/settings.json`), a snapshot export/import of
`~/.ssh/config` + `~/.lazyssh/metadata.json` (which already carries Phase 1 `private_key`), an
encrypted envelope (PBKDF2-SHA256 → AES-256-GCM over gzip(JSON)), two upload/download backends
(local file, GitHub Gist over stdlib `net/http`), a `SyncService` that orders Push/Pull so a bad
password never touches local files, and a tview Sync screen on key `y`.

## Technical Context

**Language/Version**: Go 1.26 toolchain, `go 1.26.0` directive (set in Phase 1); `crypto/pbkdf2`
(Go 1.24+) and `encoding/json` `omitzero` (Go 1.24+) are available

**Primary Dependencies**: existing `tview`/`tcell`, `zap`; stdlib `crypto/aes`, `crypto/cipher`,
`crypto/pbkdf2`, `crypto/sha256`, `crypto/rand`, `compress/gzip`, `encoding/base64`,
`encoding/json`, `net/http`. No new modules.

**Storage**: `~/.lazyssh/settings.json` (0600, dir 0700); sync target file (0600) or a secret
GitHub gist file `lazyssh-sync.enc`; Import rewrites `~/.ssh/config` (with backup) and
`~/.lazyssh/metadata.json`

**Testing**: `go test -race ./...` via `make test`; `t.TempDir()`; `net/http/httptest` for the gist
API; PBKDF2 iterations lowered in tests through the unexported `pbkdf2Iterations` var

**Target Platform**: Linux, macOS, Windows terminals (existing)

**Project Type**: TUI desktop CLI tool

**Performance Goals**: one PBKDF2 derivation (600000 iterations, ~0.2–0.5 s) per push/pull, run off
the UI goroutine

**Constraints**: password never stored or logged; token/bundle never logged; Pull = Download →
open → Import; 0600 files / 0700 dirs; stdlib only

**Scale/Scope**: 6 tasks; new packages `internal/adapters/data/settings_file`,
`internal/adapters/sync`; 15 new files (9 source, 6 test)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
|---|---|---|
| I. Hexagonal layering | Domain types (`Settings`, `SyncBundle`) import only stdlib. `SyncService` depends on `ports.SettingsRepository`, `ports.SnapshotRepository`, `ports.SyncBackend` and a `backendFor` func; the concrete factory that picks `file`/`gist` lives in `cmd/main.go` (research R4). UI uses `ports.SyncService` only. | PASS |
| II. Test-first | Tasks 4–8 each start with the failing tests named in the source plan; Task 9 (pure tview wiring) is verified manually per the source plan. Service tests use in-memory fakes for all three ports (no adapter import from `services`). | PASS |
| III. Minimal deps | stdlib only; no GitHub SDK, no extra crypto library. | PASS |
| IV. Security | settings/sync files 0600, dirs 0700; envelope is authenticated encryption; password only in memory; logs carry provider/path/gist id/status only; `raw_url` fetch does not send the token (research R6). | PASS |
| V. Simplicity | One interface per port with ≥2 implementations or a test fake (`SyncBackend`: file, gist); no backend registry, just a switch in `cmd`; no config knobs beyond the spec's settings. | PASS |

Post-design re-check: PASS; Complexity Tracking empty.

## Project Structure

### Documentation (this feature)

```text
specs/002-encrypted-sync/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── sync-formats.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/core/domain/settings.go                       # NEW Settings, SyncSettings (Task 4)
internal/core/domain/sync.go                           # NEW SyncBundle, SyncBundleVersion (Task 5)
internal/core/ports/repositories.go                    # + SettingsRepository (4), SnapshotRepository (5), SyncBackend + ErrNoRemote (7)
internal/core/ports/services.go                        # + SyncService (8)
internal/adapters/data/settings_file/settings_repo.go  # NEW NewRepository (Task 4)
internal/adapters/data/settings_file/settings_repo_test.go
internal/adapters/data/ssh_config_file/
├── snapshot.go                                        # NEW Export/Import (Task 5)
├── snapshot_test.go                                   # NEW (Task 5)
├── config_io.go                                       # extract saveRawConfig (Task 5)
└── ssh_config_file_repo.go                            # constructors return *Repository (Task 5)
internal/core/services/
├── sync_crypto.go, sync_crypto_test.go                # NEW (Task 6)
└── sync_service.go, sync_service_test.go              # NEW (Task 8)
internal/adapters/sync/
├── file_backend.go, file_backend_test.go              # NEW (Task 7)
└── gist_backend.go, gist_backend_test.go              # NEW (Task 7)
cmd/main.go                                            # wiring + backendFor (Task 8)
internal/adapters/ui/
├── tui.go                                             # NewTUI(logger, ss, sync, version, commit) (Task 8)
├── sync_form.go                                       # NEW handleSync, showSyncForm (Task 9)
├── handlers.go                                        # case 'y' (Task 9)
└── status_bar.go                                      # "y Sync" (Task 9)
README.md                                              # Features: sync + embedded keys (Task 9)
```

**Structure Decision**: Existing hexagonal layout plus two adapter packages named in the source
plan.

## Design per task

### Task 4: Settings domain and repository

- `domain.SyncSettings{ Provider, FilePath, GistID, GistToken string; LastPushAt, LastPullAt
  time.Time }`, tags `provider, file_path, gist_id, gist_token` (`omitempty`), `last_push_at,
  last_pull_at` (`omitzero`, see research R2). `domain.Settings{ Sync SyncSettings \`json:"sync"\` }`.
- `ports.SettingsRepository{ Load() (domain.Settings, error); Save(domain.Settings) error }`.
- `settings_file.NewRepository(path string) ports.SettingsRepository`: `Load` → `os.ReadFile`,
  not-exist → zero value; `Save` → `MkdirAll(dir, 0o700)`, `MarshalIndent`, `WriteFile 0o600`.
- Tests: `TestSettingsRepo_LoadMissingReturnsZero`, `TestSettingsRepo_RoundTripAndPerms`.

### Task 5: Snapshot export/import

- `domain.SyncBundle{ Version int \`json:"version"\`; ExportedAt time.Time \`json:"exported_at"\`;
  Hostname string \`json:"hostname"\`; SSHConfig string \`json:"ssh_config"\`; Metadata
  json.RawMessage \`json:"metadata"\` }`, `const SyncBundleVersion = 1`.
- `ports.SnapshotRepository{ Export() (domain.SyncBundle, error); Import(domain.SyncBundle) error }`.
- `NewRepository` / `NewRepositoryWithFS` return `*Repository` (satisfies both ports).
- `Export`: config via `fileSystem.Open` (not-exist → `""`), metadata via `os.ReadFile` (missing or
  empty → `{}`), `Hostname` from `os.Hostname()` (error ignored → `""`), `ExportedAt = time.Now()`.
- `Import`: `Version != SyncBundleVersion` → error before any write; `saveRawConfig(b.SSHConfig)`
  (extracted from `saveConfig`: `createOriginalBackupIfNeeded` + `createBackup`, both no-op when
  config missing, then temp file + rename); metadata: `ensureDirectory`, write
  `<path>.tmp` 0600, rename (research R3).
- Tests: `TestExport_ReadsConfigAndMetadata`, `TestExport_MissingFilesGiveEmpty`,
  `TestImport_WritesBothAndBacksUpConfig`, `TestImport_RejectsUnknownVersion`.

### Task 6: Bundle encryption

- `var ErrPasswordRequired, ErrBadPassword`; `var pbkdf2Iterations = 600000`.
- `sealBundle`: empty pw → `ErrPasswordRequired`; JSON → gzip → `pbkdf2.Key(sha256.New, pw, salt,
  iter, 32)` → `aes.NewCipher` → `cipher.NewGCM` → `Seal(nil, nonce, gz, nil)` → envelope JSON.
- `openBundle`: empty pw → `ErrPasswordRequired`; parse envelope; `v != 1` or `kdf !=
  "pbkdf2-sha256"` or bad salt/nonce length → format error; derive with the envelope's `iter`;
  GCM `Open` error → `ErrBadPassword`; gunzip → JSON.
- Tests: `TestSealOpen_RoundTrip`, `TestOpen_WrongPassword`, `TestSeal_EmptyPassword`,
  `TestOpen_TamperedCiphertext`, `TestSeal_NoPlaintextLeak`.

### Task 7: Sync backends (file, gist)

- `ports.SyncBackend{ Upload(ctx, blob) (string, error); Download(ctx) ([]byte, error) }`,
  `var ports.ErrNoRemote = errors.New("no remote sync data found")`.
- `sync.NewFileBackend(path)`: `~` expanded; Upload = `MkdirAll(dir, 0o700)` + `os.CreateTemp` in
  dir (0600) + write + close + rename, returns the path; Download not-exist → `ErrNoRemote`.
- `sync.NewGistBackend(token, gistID)` → `newGistBackend(token, gistID,
  "https://api.github.com", &http.Client{Timeout: 30 * time.Second})`; `const gistFileName =
  "lazyssh-sync.enc"`. Upload: POST `/gists` (`public:false`, description `lazyssh sync`) when id
  empty else PATCH `/gists/{id}`; returns the response `id`. Download: empty id → `ErrNoRemote`;
  GET `/gists/{id}`; missing file → `ErrNoRemote`; `truncated` → GET `raw_url`. Non-2xx →
  `fmt.Errorf("github: %s: %s", resp.Status, message)`.
- Tests: `TestFileBackend_RoundTrip`, `TestFileBackend_MissingIsErrNoRemote`,
  `TestGist_UploadCreatesWhenNoID`, `TestGist_UploadPatchesWhenID`, `TestGist_DownloadReadsContent`,
  `TestGist_DownloadFollowsRawURLWhenTruncated`, `TestGist_Non2xxIncludesMessage`.

### Task 8: Sync service and wiring

- `ports.SyncService{ Settings(); SaveSettings(); Push(ctx, pw); Pull(ctx, pw) }`.
- `services.NewSyncService(logger, settings, snapshot, backendFor func(domain.SyncSettings)
  (ports.SyncBackend, error)) ports.SyncService`; `var services.ErrSyncNotConfigured`.
- Empty `Provider` → `ErrSyncNotConfigured` in the service (tested); unknown values →
  `ErrSyncNotConfigured` from the `cmd` factory's default branch.
- Push: pw check → load settings → `backendFor` → Export → seal → Upload → `GistID = id` if
  provider `gist` → `LastPushAt = now` → Save. Pull: pw check → load → `backendFor` → Download →
  open → Import → `LastPullAt = now` → Save. Logs: provider + outcome only.
- `cmd/main.go`: `settings_file.NewRepository(~/.lazyssh/settings.json)`, `backendFor` switch
  (`file` → `syncadapter.NewFileBackend`, `gist` → `syncadapter.NewGistBackend`, else
  `services.ErrSyncNotConfigured`), `ui.NewTUI(log, serverService, syncService, version,
  gitCommit)`.
- Tests (fakes for all three ports): `TestSync_PushThenPull_RoundTrip`,
  `TestSync_PullWrongPassword_DoesNotImport`, `TestSync_NotConfigured`,
  `TestSync_PushSavesReturnedGistID`.

### Task 9: Sync screen

- `handleSync()` loads settings (error → red status) and calls `showSyncForm(s)`.
- `showSyncForm`: dropdown `file|gist`, `File path:`, `Gist ID:`, `GitHub token:`
  (`AddPasswordField`), `Password:` (`AddPasswordField`), buttons `Push`, `Pull`,
  `Save settings`, `Cancel`; footer TextView `Last push: … / Last pull: …` (`never` for zero);
  Esc = Cancel. Push/Pull: save form settings → `returnToMain` → `Pushing…` → goroutine with
  `context.Background()` → `QueueUpdateDraw` status (`Pushed (<p>)` / `Push failed: <err>`), Pull
  also `refreshServerList()`. Pull opens the confirm modal with the exact spec text first.
- `handlers.go`: `case 'y': t.handleSync()`; `status_bar.go`: `• [white]y[-] Sync`.
- Manual verification per quickstart.md.

## Complexity Tracking

No violations.
