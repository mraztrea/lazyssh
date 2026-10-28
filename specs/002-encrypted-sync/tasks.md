---

description: "Task list for Settings File and Encrypted Sync"
---

# Tasks: Settings File and Encrypted Sync

**Input**: Design documents from `/specs/002-encrypted-sync/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/sync-formats.md, quickstart.md

**Tests**: Required. Constitution II and the source plan mandate TDD: write the test, see it fail,
implement, see it pass, commit. Task 9 (tview wiring) is verified manually (quickstart.md).

**Organization**: Phases map 1:1 onto source plan (`docs/2026-10-28-sync-keys-sftp-plan.md`)
Tasks 4–9. Each phase ends in exactly one commit with the source plan's message, ending with the
session's `Co-Authored-By` line. Story labels show which spec user stories each task serves.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 push, US2 pull, US3 gist, US4 settings persistence

## Phase 1: Setup

No setup tasks: module, lint and test tooling exist; no new dependencies (stdlib only).

---

## Phase 2: Source Task 4 "Settings domain and repository" (US4)

**Goal**: `~/.lazyssh/settings.json` loads/saves `domain.Settings` with 0600/0700.

**Independent Test**: `go test ./internal/adapters/data/settings_file/ -v`

- [X] T001 [US4] Write failing tests `TestSettingsRepo_LoadMissingReturnsZero` (missing file → `domain.Settings{}`, nil error) and `TestSettingsRepo_RoundTripAndPerms` (path `<tmp>/sub/settings.json`; save `{Provider:"gist", GistID:"abc", GistToken:"tok"}`; load equals; file mode 0600) in new file internal/adapters/data/settings_file/settings_repo_test.go (Apache header)
- [X] T002 [US4] Run `go test ./internal/adapters/data/settings_file/ -v`; confirm FAIL (package/types missing)
- [X] T003 [P] [US4] Create internal/core/domain/settings.go (Apache header): `SyncSettings{ Provider, FilePath, GistID, GistToken string; LastPushAt, LastPullAt time.Time }` with tags `provider,omitempty`, `file_path,omitempty`, `gist_id,omitempty`, `gist_token,omitempty`, `last_push_at,omitzero`, `last_pull_at,omitzero` (research R2); `Settings{ Sync SyncSettings \`json:"sync"\` }`
- [X] T004 [P] [US4] Append `SettingsRepository{ Load() (domain.Settings, error); Save(domain.Settings) error }` to internal/core/ports/repositories.go
- [X] T005 [US4] Create internal/adapters/data/settings_file/settings_repo.go (Apache header): `NewRepository(path string) ports.SettingsRepository`; `Load` = `os.ReadFile`, `os.IsNotExist` → zero value, `json.Unmarshal`; `Save` = `os.MkdirAll(dir, 0o700)`, `json.MarshalIndent`, `os.WriteFile(path, data, 0o600)`
- [X] T006 [US4] Run `make test`; confirm PASS
- [X] T007 [US4] Commit `feat: add settings file repository` (internal/core/domain/settings.go internal/core/ports/repositories.go internal/adapters/data/settings_file)

---

## Phase 3: Source Task 5 "Snapshot export/import" (US1, US2)

**Goal**: The `ssh_config_file` repository exports config + metadata as a `SyncBundle` and imports
one with backup and atomic writes.

**Independent Test**: `go test ./internal/adapters/data/ssh_config_file/ -run 'Export|Import' -v`

- [X] T008 [US1] [US2] Write failing tests in new file internal/adapters/data/ssh_config_file/snapshot_test.go (Apache header, `t.TempDir()`, `NewRepository(zap.NewNop().Sugar(), cfg, meta)`): `TestExport_ReadsConfigAndMetadata` (cfg `"Host a\n  HostName 1.1.1.1\n"`, meta `{"a":{"tags":["x"]}}` → `Version==1`, `SSHConfig==cfg`, `string(Metadata)==meta`, `!ExportedAt.IsZero()`), `TestExport_MissingFilesGiveEmpty` (`SSHConfig==""`, `string(Metadata)=="{}"`), `TestImport_WritesBothAndBacksUpConfig` (existing cfg `"Host old\n"`; Import `{Version:1, SSHConfig:"Host new\n", Metadata:{"new":{}}}` → cfg file == `"Host new\n"`, meta file == `{"new":{}}`, a file ending in `BackupSuffix` exists in cfg dir, both files 0600), `TestImport_RejectsUnknownVersion` (`Version: 2` → error, cfg unchanged)
- [X] T009 [US1] [US2] Run `go test ./internal/adapters/data/ssh_config_file/ -run 'Export|Import' -v`; confirm FAIL
- [X] T010 [P] [US1] Create internal/core/domain/sync.go (Apache header): `const SyncBundleVersion = 1`; `SyncBundle{ Version int \`json:"version"\`; ExportedAt time.Time \`json:"exported_at"\`; Hostname string \`json:"hostname"\`; SSHConfig string \`json:"ssh_config"\`; Metadata json.RawMessage \`json:"metadata"\` }`
- [X] T011 [P] [US1] Append `SnapshotRepository{ Export() (domain.SyncBundle, error); Import(domain.SyncBundle) error }` to internal/core/ports/repositories.go
- [X] T012 [US1] In internal/adapters/data/ssh_config_file/ssh_config_file_repo.go change `NewRepository` and `NewRepositoryWithFS` return type to `*Repository`
- [X] T013 [US2] In internal/adapters/data/ssh_config_file/config_io.go extract `saveRawConfig(content string) error` from `saveConfig` (original backup + timestamped backup + temp file + `fileSystem.Rename`, order unchanged; `writeConfigToFile` takes the content string); `saveConfig(cfg)` becomes `saveRawConfig(cfg.String())` and Import reuses it (deviation from source plan name `writeRawToFile(path, content)`: the config write needs the backups too, so one helper covers both callers)
- [X] T014 [US1] [US2] Create internal/adapters/data/ssh_config_file/snapshot.go (Apache header): `Export` (config via `fileSystem.Open`/`IsNotExist` → `""`; metadata via `os.ReadFile`, missing or empty → `{}`; `os.Hostname()`; `time.Now()`); `Import` (version check first → `fmt.Errorf("unsupported sync bundle version %d", v)`; `saveRawConfig(b.SSHConfig)` (backups + atomic replace); metadata: `metadataManager.ensureDirectory()`, write `<path>.tmp` 0600, `os.Rename`; `ponytail:` comment on the two-rename ceiling, research R3); log paths only
- [X] T015 [US1] [US2] Run `make test`; confirm PASS
- [X] T016 [US1] [US2] Commit `feat: export and import config snapshot for sync` (internal/core internal/adapters/data/ssh_config_file)

---

## Phase 4: Source Task 6 "Bundle encryption" (US1, US2)

**Goal**: `sealBundle` / `openBundle` implement the envelope in contracts/sync-formats.md.

**Independent Test**: `go test ./internal/core/services/ -run 'Seal|Open' -v`

- [X] T017 [US1] [US2] Write failing tests in new file internal/core/services/sync_crypto_test.go (Apache header; `TestMain` or per-test sets `pbkdf2Iterations = 1000` for speed): `TestSealOpen_RoundTrip` (bundle `{Version:1, SSHConfig:"Host a\n", Metadata:{"a":{}}}`, pw `"pw"` → equal SSHConfig), `TestOpen_WrongPassword` (`errors.Is(err, ErrBadPassword)`), `TestSeal_EmptyPassword` (`errors.Is(err, ErrPasswordRequired)`, also for `openBundle`), `TestOpen_TamperedCiphertext` (flip one byte of decoded `data`, re-encode → error), `TestSeal_NoPlaintextLeak` (`!bytes.Contains(blob, []byte("Host a"))`; envelope has `v==1`, `kdf=="pbkdf2-sha256"`, 16-byte salt, 12-byte nonce)
- [X] T018 [US1] [US2] Run `go test ./internal/core/services/ -run 'Seal|Open' -v`; confirm FAIL
- [X] T019 [US1] [US2] Create internal/core/services/sync_crypto.go (Apache header): `ErrPasswordRequired = errors.New("password is required")`, `ErrBadPassword = errors.New("wrong password or corrupted sync data")`, `var pbkdf2Iterations = 600000`; envelope struct (`v`, `kdf`, `iter`, `salt`, `nonce`, `data`; `[]byte` fields base64 via encoding/json); `sealBundle` (empty pw check → json → gzip → random 16B salt + 12B nonce via `crypto/rand` → `pbkdf2.Key(sha256.New, pw, salt, iter, 32)` → AES-GCM Seal → envelope JSON); `openBundle` (empty pw check → parse → reject `v != 1`, `kdf != "pbkdf2-sha256"`, wrong salt/nonce length → derive with envelope `iter` → GCM Open error → `ErrBadPassword` → gunzip → json)
- [X] T020 [US1] [US2] Run `make test`; confirm PASS
- [X] T021 [US1] [US2] Commit `feat: encrypt sync bundles with AES-GCM and PBKDF2` (internal/core/services/sync_crypto.go internal/core/services/sync_crypto_test.go)

---

## Phase 5: Source Task 7 "Sync backends (file, gist)" (US1, US2, US3)

**Goal**: File and GitHub Gist upload/download over stdlib only.

**Independent Test**: `go test ./internal/adapters/sync/ -v`

- [X] T022 [P] [US1] [US2] Write failing tests in new file internal/adapters/sync/file_backend_test.go (Apache header): `TestFileBackend_RoundTrip` (Upload returns the path; Download equals blob; mode 0600), `TestFileBackend_MissingIsErrNoRemote` (`errors.Is(err, ports.ErrNoRemote)`)
- [X] T023 [P] [US3] Write failing tests in new file internal/adapters/sync/gist_backend_test.go (Apache header, `httptest.Server`, `newGistBackend(token, id, srv.URL, srv.Client())`): `TestGist_UploadCreatesWhenNoID` (POST `/gists`, `Authorization: Bearer tok`, body `public==false`, `description=="lazyssh sync"`, `files["lazyssh-sync.enc"].content == string(blob)`; 201 `{"id":"g1"}` → returns `"g1"`), `TestGist_UploadPatchesWhenID` (PATCH `/gists/g1`), `TestGist_DownloadReadsContent` (GET `/gists/g1` → content), `TestGist_DownloadFollowsRawURLWhenTruncated` (`truncated:true`, `raw_url` → second handler path `/raw`; result is the full body; raw request has no Authorization header), `TestGist_Non2xxIncludesMessage` (401 `{"message":"Bad credentials"}` → err contains `Bad credentials` and `401`, not the token)
- [X] T024 [US1] [US3] Run `go test ./internal/adapters/sync/ -v`; confirm FAIL
- [X] T025 [US1] Append to internal/core/ports/repositories.go: `SyncBackend{ Upload(ctx context.Context, blob []byte) (remoteID string, err error); Download(ctx context.Context) ([]byte, error) }` and `var ErrNoRemote = errors.New("no remote sync data found")`
- [X] T026 [P] [US1] [US2] Create internal/adapters/sync/file_backend.go (Apache header, package `sync`): `NewFileBackend(path string) ports.SyncBackend` (expand leading `~` / `~/` via `os.UserHomeDir`); Upload = `MkdirAll(dir, 0o700)`, `os.CreateTemp(dir, ".lazyssh-sync-*")` (0600), write, close, `os.Rename`, return path; Download = `os.ReadFile`, not-exist → `ports.ErrNoRemote`
- [X] T027 [P] [US3] Create internal/adapters/sync/gist_backend.go (Apache header): `const gistFileName = "lazyssh-sync.enc"`; `NewGistBackend(token, gistID string) ports.SyncBackend` → `newGistBackend(token, gistID, "https://api.github.com", &http.Client{Timeout: 30 * time.Second})`; Upload POST `/gists` (`{"description":"lazyssh sync","public":false,"files":{...}}`) when id empty else PATCH `/gists/{id}` (`{"files":{...}}`), returns response `id`; Download: empty id → `ErrNoRemote`, GET `/gists/{id}`, missing file → `ErrNoRemote`, `truncated` → GET `raw_url` without Authorization; headers `Accept: application/vnd.github+json`, `Authorization: Bearer <token>`; non-2xx → `fmt.Errorf("github: %s: %s", resp.Status, message)`; requests built with `http.NewRequestWithContext`
- [X] T028 [US1] [US3] Run `make test`; confirm PASS
- [X] T029 [US1] [US3] Commit `feat: add file and GitHub gist sync backends` (internal/core/ports internal/adapters/sync)

---

## Phase 6: Source Task 8 "Sync service and wiring" (US1, US2, US3) 🎯 MVP

**Goal**: `SyncService` orders Push/Pull; app constructs it; wrong password never imports.

**Independent Test**: `go test ./internal/core/services/ -run TestSync -v`

- [X] T030 [US1] [US2] [US3] Write failing tests in new file internal/core/services/sync_service_test.go (Apache header; fakes: settings repo in memory, snapshot struct with `bundle`/`imported`, backend holding `[]byte` and a fixed remote id; `pbkdf2Iterations` lowered): `TestSync_PushThenPull_RoundTrip` (push from snapshot A, pull into snapshot B → equal SSHConfig; `LastPushAt`/`LastPullAt` non-zero), `TestSync_PullWrongPassword_DoesNotImport` (`imported == false`, `errors.Is(err, ErrBadPassword)`), `TestSync_NotConfigured` (Provider `""` → Push and Pull return `ErrSyncNotConfigured` and `backendFor` is never called), `TestSync_PushSavesReturnedGistID` (backend returns `"g1"`, Provider `"gist"` → saved `GistID == "g1"`)
- [X] T031 [US1] Run `go test ./internal/core/services/ -run TestSync -v`; confirm FAIL
- [X] T032 [US1] Append `SyncService{ Settings() (domain.Settings, error); SaveSettings(domain.Settings) error; Push(ctx context.Context, password string) error; Pull(ctx context.Context, password string) error }` to internal/core/ports/services.go
- [X] T033 [US1] [US2] Create internal/core/services/sync_service.go (Apache header): `ErrSyncNotConfigured = errors.New("sync is not configured: choose provider file or gist")`; `NewSyncService(logger *zap.SugaredLogger, settings ports.SettingsRepository, snapshot ports.SnapshotRepository, backendFor func(domain.SyncSettings) (ports.SyncBackend, error)) ports.SyncService`; shared `backend()` step: Load settings, Provider `""` → `ErrSyncNotConfigured`, else `backendFor(s.Sync)`; Push (empty pw → `ErrPasswordRequired`; Load; backendFor; Export; sealBundle; Upload; `GistID = id` when provider `gist`; `LastPushAt = time.Now().UTC()`; Save); Pull (empty pw → `ErrPasswordRequired`; Load; backendFor; Download; openBundle; Import; `LastPullAt`; Save); logs: provider and error only
- [X] T034 [US1] In internal/adapters/ui/tui.go change to `NewTUI(logger *zap.SugaredLogger, ss ports.ServerService, sync ports.SyncService, version, commit string) App`; store as `syncService`
- [X] T035 [US1] In cmd/main.go build `settings_file.NewRepository(filepath.Join(home, ".lazyssh", "settings.json"))`, `backendFor` (`"file"` → `syncadapter.NewFileBackend(s.FilePath)`, `"gist"` → `syncadapter.NewGistBackend(s.GistToken, s.GistID)`, else `services.ErrSyncNotConfigured`), `services.NewSyncService(log, settingsRepo, serverRepo, backendFor)`, pass to `ui.NewTUI`; `serverRepo` is now `*ssh_config_file.Repository`
- [X] T036 [US1] Run `make test && make build`; confirm PASS
- [X] T037 [US1] Commit `feat: add sync service with push and pull` (cmd internal)

---

## Phase 7: Source Task 9 "Sync screen" (US1, US2, US3, US4)

**Goal**: Key `y` opens the Sync form; Push/Pull run in background with status feedback.

**Independent Test**: quickstart.md manual steps 1–4 under a throwaway `HOME` in tmux.

- [X] T038 [US1] [US2] [US3] [US4] Create internal/adapters/ui/sync_form.go (Apache header): `func (t *tui) handleSync()` (load `t.syncService.Settings()`, error → red status) and `func (t *tui) showSyncForm(s domain.Settings)` with `Provider:` dropdown (`file`, `gist`), `File path:` (width 50), `Gist ID:` (width 40), `GitHub token:` `AddPasswordField`, `Password:` `AddPasswordField` (empty), buttons `Push`, `Pull`, `Save settings`, `Cancel`, footer TextView `Last push: <time or never> / Last pull: <time or never>`, Esc → `returnToMain`; Push/Pull save form settings (keeping `LastPushAt`/`LastPullAt`), Pull first shows modal with exact text `Pull will REPLACE ~/.ssh/config and ~/.lazyssh/metadata.json (backup is taken). Continue?`; then `returnToMain`, status `Pushing…`/`Pulling…`, goroutine, `QueueUpdateDraw` → `Pushed (<p>)` / `Pulled (<p>)` green + `refreshServerList()` after Pull, or `Push failed: <err>` / `Pull failed: <err>` in `#FF6B6B`
- [X] T039 [P] [US1] In internal/adapters/ui/handlers.go `handleGlobalKeys` add `case 'y': t.handleSync(); return nil`
- [X] T040 [P] [US1] In internal/adapters/ui/status_bar.go append `  • [white]y[-] Sync` before Quit in `DefaultStatusText`
- [X] T041 [P] [US1] In README.md Features list add encrypted sync (file / GitHub Gist) and embedded private keys
- [X] T042 [US1] [US2] `make build`; manual verification per quickstart.md (tmux, throwaway HOME, provider `file`, `/tmp/lazyssh-sync.enc`): push, confirm no `Host` plaintext, edit server, pull, confirm edit reverted and new `*lazyssh.backup`
- [X] T043 [US1] Run `make build && make test`; confirm PASS
- [X] T044 [US1] Commit `feat: add sync screen for push and pull` (internal/adapters/ui README.md)

---

## Phase 8: Polish & Cross-Cutting

- [X] T045 Grep the diff for log/status/error calls that could include the token, password or bundle contents (FR-019); fix any
- [X] T046 Mark tasks done, set spec Status to Implemented, commit `docs: add speckit artifacts for encrypted sync` (specs/002-encrypted-sync; `.specify/feature.json` is git-ignored)
- [X] T047 `git checkout feat/sync-keys-sftp && git merge --ff-only 002-encrypted-sync`; tree clean

---

## Dependencies & Execution Order

- Phase 2 (Task 4) and Phase 3 (Task 5) are independent; Phase 4 (Task 6) needs `domain.SyncBundle`
  from Phase 3; Phase 5 (Task 7) needs only `ports`; Phase 6 (Task 8) needs 2–5; Phase 7 (Task 9)
  needs 6. Run in source-plan order for one commit per task.
- Within a phase: test → see fail → implement → see pass → commit.

### Parallel Opportunities

- T003/T004, T010/T011, T022/T023, T026/T027, T039/T040/T041 touch different files.

## Implementation Strategy

MVP = Phases 2–6: push/pull work through the service (testable without UI). Phase 7 makes it
reachable from the TUI. Ship together on `feat/sync-keys-sftp`.

## Traceability

| Spec | Tasks |
|---|---|
| FR-001, FR-002, FR-003 | T001–T005, T038 |
| FR-004, FR-005, FR-006 | T008–T014 |
| FR-007, FR-008, FR-009 | T017–T019, T033 |
| FR-010 | T022, T026 |
| FR-011 | T023, T027 |
| FR-012 | T030, T033, T035 |
| FR-013, FR-014 | T030, T033 |
| FR-015, FR-016, FR-017, FR-018 | T038–T040, T042 |
| FR-019 | T014, T027, T033, T045 |
| FR-020 | T001, T005, T022, T026 |
