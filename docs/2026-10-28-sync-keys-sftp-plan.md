# Config Sync, Embedded Keys, SFTP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add embedded private keys, encrypted settings/config sync (file or GitHub Gist), and an SFTP dual-pane file manager to lazyssh.

**Architecture:** Keep the existing hexagonal split. New domain types in `internal/core/domain`, new interfaces in `internal/core/ports`, logic in `internal/core/services`, file/HTTP adapters under `internal/adapters`, tview screens in `internal/adapters/ui`. SFTP reuses the system `ssh` binary (`ssh -s sftp alias`) piped into `pkg/sftp`, so every SSH config option keeps working.

**Tech Stack:** Go 1.24, tview/tcell, `github.com/pkg/sftp`, `golang.org/x/crypto/ssh` (key parsing only), stdlib `crypto/aes`, `crypto/cipher`, `crypto/pbkdf2`, `net/http`.

**Spec:** `docs/2026-10-28-sync-keys-sftp-spec.md`

## Global Constraints

- Go 1.24+. `make build` (includes lint) and `make test` pass after every task.
- Only new deps: `github.com/pkg/sftp v1.13.12`, `golang.org/x/crypto v0.58.0`.
- Files under `~/.lazyssh` are 0600, directories 0700.
- Private key text never appears in logs, status bar, or details pane.
- Apache header on every new `.go` file (copy from any existing file).
- Commit prefixes: `feat:`, `fix:`, `improve:`, `test:`, `docs:`.
- Alias charset is already `^[A-Za-z0-9_.-]+$`; additionally reject `.` and `..`.

## Review Focus

Inputs the spec implies that a user will hit. Each has its test pinned to the owning task.

1. Key pasted from Windows with `\r\n` line endings must be accepted and stored with `\n` only. (Task 1)
2. Pull with the wrong password must leave `~/.ssh/config` and `metadata.json` untouched. (Task 8)
3. Gist response with `truncated: true` must fall back to `raw_url`, or a big bundle silently corrupts. (Task 7)
4. `ssh` exiting immediately (auth failure, unknown host) must make `Connect` return an error, not hang. (Task 10)
5. Renaming a server that has an embedded key must move the key file and rewrite `IdentityFile`. (Task 2)

---

# Phase 1: Embedded private keys

### Task 1: Domain field, metadata field, validation

**Files:**
- Modify: `internal/core/domain/server.go` (add field after `IdentityFiles`)
- Modify: `internal/adapters/data/ssh_config_file/metadata_manager.go:28-33` (struct), `:103-125` (`updateServer`)
- Modify: `internal/adapters/data/ssh_config_file/mapper.go:295-320` (`mergeMetadata`)
- Modify: `internal/core/services/server_service.go:77-110` (`validateServer`)
- Create: `internal/core/services/server_service_test.go`
- Modify: `go.mod` (`go get golang.org/x/crypto@v0.58.0`)

**Interfaces:**
- Produces: `domain.Server.PrivateKey string`; `ServerMetadata.PrivateKey string \`json:"private_key,omitempty"\``; `normalizePrivateKey(s string) string` (services, unexported) that converts `\r\n`→`\n`, trims trailing whitespace, appends one `\n`, returns `""` for blank input.

- [ ] **Step 1: Write the failing tests**

```go
func TestValidateServer_PrivateKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, _ := ssh.MarshalPrivateKey(priv, "")
	validPEM := string(pem.EncodeToMemory(block))
	base := domain.Server{Alias: "a", Host: "1.2.3.4"}

	cases := []struct{ name, key string; wantErr bool }{
		{"empty ok", "", false},
		{"valid ed25519", validPEM, false},
		{"crlf accepted", strings.ReplaceAll(validPEM, "\n", "\r\n"), false},
		{"garbage", "not a key", true},
	}
	for _, c := range cases {
		s := base; s.PrivateKey = c.key
		if err := validateServer(s); (err != nil) != c.wantErr { t.Errorf("%s: got %v", c.name, err) }
	}
}

func TestValidateServer_RejectsDotAliases(t *testing.T) {
	for _, a := range []string{".", ".."} {
		if validateServer(domain.Server{Alias: a, Host: "h"}) == nil { t.Errorf("alias %q accepted", a) }
	}
}

func TestNormalizePrivateKey(t *testing.T) {
	if got := normalizePrivateKey("a\r\nb\r\n\r\n"); got != "a\nb\n" { t.Fatalf("got %q", got) }
	if got := normalizePrivateKey("  \n"); got != "" { t.Fatalf("got %q", got) }
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/core/services/ -run 'TestValidateServer|TestNormalize' -v`
Expected: FAIL (compile error: `PrivateKey` undefined / `normalizePrivateKey` undefined)

- [ ] **Step 3: Implement**

Add `PrivateKey string` to `domain.Server`. Add `PrivateKey` to `ServerMetadata`; in `updateServer` set `merged.PrivateKey = server.PrivateKey` (unconditionally, so clearing works); in `mergeMetadata` set `servers[i].PrivateKey = meta.PrivateKey`. In `validateServer`: reject alias `.`/`..`; if `PrivateKey != ""`, run `normalizePrivateKey` then `ssh.ParseRawPrivateKey([]byte(key))`; accept `*ssh.PassphraseMissingError` via `errors.As`, any other error → `fmt.Errorf("private key is not a valid OpenSSH/PEM private key")`. In `AddServer`/`UpdateServer` services, assign `server.PrivateKey = normalizePrivateKey(server.PrivateKey)` before calling the repository.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/core/services/ -v` then `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/core internal/adapters/data
git commit -m "feat: add embedded private key field with validation"
```

### Task 2: Key file materialisation in the repository

**Files:**
- Create: `internal/adapters/data/ssh_config_file/key_store.go`
- Create: `internal/adapters/data/ssh_config_file/key_store_test.go`
- Modify: `internal/adapters/data/ssh_config_file/ssh_config_file_repo.go` (`Repository` struct, `NewRepository`, `NewRepositoryWithFS`, `ListServers`, `AddServer`, `UpdateServer`, `DeleteServer`)

**Interfaces:**
- Consumes: `domain.Server.PrivateKey`, `ServerMetadata.PrivateKey` (Task 1).
- Produces (unexported, package `ssh_config_file`):
  - `type keyStore struct{ dir string }` where `dir = filepath.Join(filepath.Dir(metaDataPath), "keys")`
  - `func (k keyStore) path(alias string) string`
  - `func (k keyStore) write(alias, pem string) error` (MkdirAll 0700, WriteFile 0600)
  - `func (k keyStore) remove(alias string) error` (ignore not-exist)
  - `func (k keyStore) ensure(alias, pem string) error` (write only if missing or content differs)
  - `func withKeyPath(files []string, path string) []string` (prepend if absent)
  - `func withoutKeyPath(files []string, path string) []string`

- [ ] **Step 1: Write the failing tests** (use `t.TempDir()` for both config and metadata paths; `NewRepository(zap.NewNop().Sugar(), cfg, meta)`)

```go
func TestAddServer_WithPrivateKey_WritesFileAndIdentityFile(t *testing.T) {
	// AddServer{Alias:"web", Host:"h", PrivateKey:"-----BEGIN X-----\nabc\n-----END X-----\n"}
	// assert: <tmp>/keys/web exists, mode 0600, content == PrivateKey
	// assert: ListServers("")[0].IdentityFiles[0] == <tmp>/keys/web
	// assert: ListServers("")[0].PrivateKey == PrivateKey
}
func TestUpdateServer_ClearPrivateKey_RemovesFileAndIdentityFile(t *testing.T) {}
func TestUpdateServer_RenameAlias_MovesKeyFile(t *testing.T) {
	// rename web -> web2: keys/web gone, keys/web2 exists, IdentityFiles == [keys/web2]
}
func TestDeleteServer_RemovesKeyFile(t *testing.T) {}
func TestListServers_RecreatesMissingKeyFile(t *testing.T) {
	// add with key, os.Remove(keys/web), ListServers -> file exists again
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/adapters/data/ssh_config_file/ -run 'PrivateKey|KeyFile' -v`
Expected: FAIL (files not created)

- [ ] **Step 3: Implement**

Add `keys keyStore` to `Repository`, initialised in both constructors. In `AddServer`: if `PrivateKey != ""`, `keys.write`, `server.IdentityFiles = withKeyPath(...)` before `createHostFromServer`. In `UpdateServer`: compute `oldPath := keys.path(server.Alias)`, `newPath := keys.path(newServer.Alias)`; cases: key present → `remove(old)` if alias changed, `write(new)`, `IdentityFiles = withKeyPath(withoutKeyPath(files, oldPath), newPath)`; key empty → `remove(old)`, `IdentityFiles = withoutKeyPath(files, oldPath)`. In `DeleteServer`: `keys.remove(alias)` after metadata delete. In `ListServers` after `mergeMetadata`: for each server with `PrivateKey != ""`, `keys.ensure(alias, PrivateKey)` (log warn on error, do not fail the list).

- [ ] **Step 4: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/data/ssh_config_file
git commit -m "feat: materialise embedded private keys to ~/.lazyssh/keys"
```

### Task 3: Form, details, help for embedded key

**Files:**
- Modify: `internal/adapters/ui/server_form.go` (`ServerFormData` at `:1641`, `getDefaultValues` `:1066`, `createAuthenticationForm` `:1477`, `getFormData` `:1738`, `dataToServer` `:2142`)
- Modify: `internal/adapters/ui/server_details.go:65` (Key line)
- Modify: `internal/adapters/ui/field_help.go` (add `"PrivateKey"` entry next to `"IdentityAgent"` at `:547`)
- Create: `internal/adapters/ui/private_key_test.go`
- Create: `internal/adapters/ui/private_key.go`

**Interfaces:**
- Consumes: `domain.Server.PrivateKey` (Task 1).
- Produces: `func resolvePrivateKey(text, importPath string) (string, error)` in `private_key.go`: returns `text` if non-blank; else if `importPath` non-blank, expands leading `~` with `os.UserHomeDir`, reads file, returns content; else `""`. `ServerFormData` gains `PrivateKey string` and `PrivateKeyImportPath string`.

- [ ] **Step 1: Write the failing test**

```go
func TestResolvePrivateKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "k"); os.WriteFile(p, []byte("KEY\n"), 0o600)
	if got, _ := resolvePrivateKey("pasted", p); got != "pasted" { t.Fatal("text must win") }
	if got, _ := resolvePrivateKey("", p); got != "KEY\n" { t.Fatalf("got %q", got) }
	if _, err := resolvePrivateKey("", filepath.Join(dir, "missing")); err == nil { t.Fatal("want error") }
	if got, _ := resolvePrivateKey("", ""); got != "" { t.Fatal("want empty") }
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/adapters/ui/ -run TestResolvePrivateKey -v`
Expected: FAIL (undefined)

- [ ] **Step 3: Implement**

`private_key.go`: `resolvePrivateKey` as specified. In `createAuthenticationForm`, before the Public Key section add `form.AddTextView("\n[yellow]▶ Embedded Private Key (synced)[-]", ...)`, `form.AddTextArea("PrivateKey:", defaultValues.PrivateKey, 60, 8, 0, nil)`, and an `addInputFieldWithHelp(form, "Import key from file:", "PrivateKeyImportPath", "", 40, "~/.ssh/id_ed25519")` with `SetAutocompleteFunc(sf.createSSHKeyAutocomplete())`. In `getFormData`: add a `getTextAreaText(fieldName)` helper mirroring `getFieldText` for `*tview.TextArea`; fill both new fields. In `handleSave`, after `data := sf.getFormData()`: `pk, err := resolvePrivateKey(data.PrivateKey, data.PrivateKeyImportPath)`; on error show a `tview.Modal` with `"Import key failed: " + err.Error()` (same pattern as the validation modal above it) and return `false`; otherwise `data.PrivateKey = pk`. In `dataToServer`: `PrivateKey: data.PrivateKey`. `getDefaultValues` sets `PrivateKey: sf.original.PrivateKey`. Details: `serverKey += " (embedded)"` when `server.PrivateKey != ""`. Help entry: Field `PrivateKey`, Category `Authentication`, Description "Private key text stored in ~/.lazyssh/metadata.json and written to ~/.lazyssh/keys/<alias>. Included in sync."

- [ ] **Step 4: Run tests and manual check**

Run: `make test && go run ./cmd` → `a`, Authentication tab, paste a key, Save, `e` again: key text shown; details pane shows `(embedded)`.
Expected: PASS, file `~/.lazyssh/keys/<alias>` exists.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/ui
git commit -m "feat: edit embedded private key from the server form"
```

---

# Phase 2: Settings and encrypted sync

### Task 4: Settings domain and repository

**Files:**
- Create: `internal/core/domain/settings.go`
- Modify: `internal/core/ports/repositories.go` (append `SettingsRepository`)
- Create: `internal/adapters/data/settings_file/settings_repo.go`
- Create: `internal/adapters/data/settings_file/settings_repo_test.go`

**Interfaces:**
- Produces:
  - `domain.SyncSettings{ Provider, FilePath, GistID, GistToken string; LastPushAt, LastPullAt time.Time }` with json tags `provider, file_path, gist_id, gist_token, last_push_at, last_pull_at` (all `omitempty`).
  - `domain.Settings{ Sync SyncSettings \`json:"sync"\` }`
  - `ports.SettingsRepository{ Load() (domain.Settings, error); Save(domain.Settings) error }`
  - `settings_file.NewRepository(path string) ports.SettingsRepository`

- [ ] **Step 1: Write the failing test**

```go
func TestSettingsRepo_LoadMissingReturnsZero(t *testing.T) {
	r := NewRepository(filepath.Join(t.TempDir(), "settings.json"))
	s, err := r.Load(); if err != nil || s != (domain.Settings{}) { t.Fatal(s, err) }
}
func TestSettingsRepo_RoundTripAndPerms(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "settings.json")
	r := NewRepository(p)
	in := domain.Settings{Sync: domain.SyncSettings{Provider: "gist", GistID: "abc", GistToken: "tok"}}
	if err := r.Save(in); err != nil { t.Fatal(err) }
	out, _ := r.Load(); if out != in { t.Fatalf("%+v", out) }
	fi, _ := os.Stat(p); if fi.Mode().Perm() != 0o600 { t.Fatal(fi.Mode()) }
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/adapters/data/settings_file/ -v`
Expected: FAIL (package missing)

- [ ] **Step 3: Implement**

`Load`: `os.ReadFile`; `os.IsNotExist` → zero value; `json.Unmarshal`. `Save`: `MkdirAll(dir, 0o700)`, `json.MarshalIndent`, `os.WriteFile(path, data, 0o600)`.

- [ ] **Step 4: Run tests**

Run: `make test` → PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/domain/settings.go internal/core/ports/repositories.go internal/adapters/data/settings_file
git commit -m "feat: add settings file repository"
```

### Task 5: Snapshot export/import

**Files:**
- Create: `internal/core/domain/sync.go`
- Modify: `internal/core/ports/repositories.go` (append `SnapshotRepository`)
- Create: `internal/adapters/data/ssh_config_file/snapshot.go`
- Create: `internal/adapters/data/ssh_config_file/snapshot_test.go`

**Interfaces:**
- Produces:
  - `domain.SyncBundle{ Version int; ExportedAt time.Time; Hostname string; SSHConfig string; Metadata json.RawMessage }` with json tags from the spec. `const domain.SyncBundleVersion = 1`.
  - `ports.SnapshotRepository{ Export() (domain.SyncBundle, error); Import(domain.SyncBundle) error }`
  - `*ssh_config_file.Repository` implements it. Change `NewRepository`/`NewRepositoryWithFS` return type to `*Repository` (it satisfies both `ports.ServerRepository` and `ports.SnapshotRepository`; `cmd/main.go` keeps compiling).

- [ ] **Step 1: Write the failing test**

```go
func TestExport_ReadsConfigAndMetadata(t *testing.T) {
	// write cfg "Host a\n  HostName 1.1.1.1\n", meta `{"a":{"tags":["x"]}}`
	// b, _ := repo.Export(); assert b.Version==1, b.SSHConfig==cfg text, string(b.Metadata)==meta text, !b.ExportedAt.IsZero()
}
func TestExport_MissingFilesGiveEmpty(t *testing.T) { /* SSHConfig=="" && string(Metadata)=="{}" */ }
func TestImport_WritesBothAndBacksUpConfig(t *testing.T) {
	// existing cfg "Host old\n"; Import{SSHConfig:"Host new\n", Metadata:[]byte(`{"new":{}}`)}
	// assert cfg file == "Host new\n", meta file == `{"new":{}}`, a file whose name ends with BackupSuffix ("lazyssh.backup") exists in cfg dir, perms 0600
}
func TestImport_RejectsUnknownVersion(t *testing.T) { /* Version: 2 -> error */ }
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/adapters/data/ssh_config_file/ -run 'Export|Import' -v` → FAIL

- [ ] **Step 3: Implement**

`Export`: read files (missing config → `""`, missing/empty metadata → `[]byte("{}")`), `Hostname` from `os.Hostname()`. `Import`: validate `Version == SyncBundleVersion`; `createOriginalBackupIfNeeded()` + `createBackup()` (skip both if config does not exist); write config via `createTempFile` + `fileSystem.Rename` pattern already in `config_io.go` (extract `writeRawToFile(path, content string) error` and reuse); write metadata with `os.WriteFile(metadataManager.filePath, …, 0o600)` after `ensureDirectory()`.

- [ ] **Step 4: Run tests** → `make test` PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core internal/adapters/data/ssh_config_file
git commit -m "feat: export and import config snapshot for sync"
```

### Task 6: Bundle encryption

**Files:**
- Create: `internal/core/services/sync_crypto.go`
- Create: `internal/core/services/sync_crypto_test.go`

**Interfaces:**
- Produces (package `services`):
  - `var ErrPasswordRequired, ErrBadPassword error`
  - `func sealBundle(b domain.SyncBundle, password string) ([]byte, error)`
  - `func openBundle(blob []byte, password string) (domain.SyncBundle, error)`
  - Envelope JSON `{v:1, kdf:"pbkdf2-sha256", iter:600000, salt, nonce, data}`; `data` = AES-256-GCM over gzip(JSON bundle); key = `pbkdf2.Key(sha256.New, password, salt, iter, 32)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestSealOpen_RoundTrip(t *testing.T) {
	in := domain.SyncBundle{Version: 1, SSHConfig: "Host a\n", Metadata: []byte(`{"a":{}}`)}
	blob, err := sealBundle(in, "pw"); if err != nil { t.Fatal(err) }
	out, err := openBundle(blob, "pw"); if err != nil || out.SSHConfig != in.SSHConfig { t.Fatal(err, out) }
}
func TestOpen_WrongPassword(t *testing.T) { /* errors.Is(err, ErrBadPassword) */ }
func TestSeal_EmptyPassword(t *testing.T) { /* errors.Is(err, ErrPasswordRequired) */ }
func TestOpen_TamperedCiphertext(t *testing.T) { /* flip one byte of decoded data -> error */ }
func TestSeal_NoPlaintextLeak(t *testing.T) { /* !bytes.Contains(blob, []byte("Host a")) */ }
```

- [ ] **Step 2: Run** `go test ./internal/core/services/ -run 'Seal|Open' -v` → FAIL

- [ ] **Step 3: Implement** as specified; use `crypto/rand` for salt (16B) and nonce (12B); GCM `Open` failure maps to `ErrBadPassword`. Tests may lower `iter` via an unexported package var `pbkdf2Iterations` (default 600000) to keep the suite fast.

- [ ] **Step 4: Run** `make test` → PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/services/sync_crypto.go internal/core/services/sync_crypto_test.go
git commit -m "feat: encrypt sync bundles with AES-GCM and PBKDF2"
```

### Task 7: Sync backends (file, gist)

**Files:**
- Modify: `internal/core/ports/repositories.go` (append `SyncBackend`)
- Create: `internal/adapters/sync/file_backend.go`, `internal/adapters/sync/gist_backend.go`
- Create: `internal/adapters/sync/file_backend_test.go`, `internal/adapters/sync/gist_backend_test.go`

**Interfaces:**
- Produces:
  - `ports.SyncBackend{ Upload(ctx context.Context, blob []byte) (remoteID string, err error); Download(ctx context.Context) ([]byte, error) }`
  - `var ports.ErrNoRemote = errors.New("no remote sync data found")`
  - `sync.NewFileBackend(path string) ports.SyncBackend` (expands leading `~`)
  - `sync.NewGistBackend(token, gistID string) ports.SyncBackend`; unexported `newGistBackend(token, gistID, baseURL string, client *http.Client)` for tests. `const gistFileName = "lazyssh-sync.enc"`.

- [ ] **Step 1: Write the failing tests**

```go
func TestFileBackend_RoundTrip(t *testing.T) { /* Upload returns path; Download equal; perms 0600 */ }
func TestFileBackend_MissingIsErrNoRemote(t *testing.T) {}

func TestGist_UploadCreatesWhenNoID(t *testing.T) {
	// httptest: POST /gists -> 201 {"id":"g1"}; assert method, Authorization "Bearer tok",
	// body.files["lazyssh-sync.enc"].content == string(blob) (the envelope is ASCII JSON, no extra encoding)
	// assert returned id == "g1"
}
func TestGist_UploadPatchesWhenID(t *testing.T) { /* PATCH /gists/g1 */ }
func TestGist_DownloadReadsContent(t *testing.T) { /* GET /gists/g1 -> files.content */ }
func TestGist_DownloadFollowsRawURLWhenTruncated(t *testing.T) { /* truncated:true, raw_url -> second handler */ }
func TestGist_Non2xxIncludesMessage(t *testing.T) { /* 401 {"message":"Bad credentials"} -> err contains "Bad credentials" */ }
```

- [ ] **Step 2: Run** `go test ./internal/adapters/sync/ -v` → FAIL

- [ ] **Step 3: Implement** per spec; HTTP client timeout 30s; headers `Accept: application/vnd.github+json`, `Authorization: Bearer <token>`.

- [ ] **Step 4: Run** `make test` → PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/ports internal/adapters/sync
git commit -m "feat: add file and GitHub gist sync backends"
```

### Task 8: Sync service and wiring

**Files:**
- Modify: `internal/core/ports/services.go` (append `SyncService`)
- Create: `internal/core/services/sync_service.go`, `internal/core/services/sync_service_test.go`
- Modify: `cmd/main.go` (construct settings repo at `~/.lazyssh/settings.json`, sync service; pass to `ui.NewTUI`)
- Modify: `internal/adapters/ui/tui.go` (`NewTUI(logger, ss ports.ServerService, sync ports.SyncService, version, commit string)`, store `syncService`)

**Interfaces:**
- Consumes: Tasks 4–7.
- Produces:
  - `ports.SyncService{ Settings() (domain.Settings, error); SaveSettings(domain.Settings) error; Push(ctx, password string) error; Pull(ctx, password string) error }`
  - `services.NewSyncService(logger, settings ports.SettingsRepository, snapshot ports.SnapshotRepository, backendFor func(domain.SyncSettings) (ports.SyncBackend, error)) ports.SyncService`
  - `services.DefaultBackendFactory(s domain.SyncSettings) (ports.SyncBackend, error)` lives in `cmd/main.go` wiring (returns `sync.NewFileBackend` / `sync.NewGistBackend`, else `ErrSyncNotConfigured`). `var services.ErrSyncNotConfigured`.

- [ ] **Step 1: Write the failing tests** (fake backend = in-memory `[]byte`; fake snapshot = struct holding a bundle; settings repo = real one on `t.TempDir()`)

```go
func TestSync_PushThenPull_RoundTrip(t *testing.T) { /* Pull into second fake snapshot; equal SSHConfig; LastPushAt/LastPullAt set */ }
func TestSync_PullWrongPassword_DoesNotImport(t *testing.T) { /* snapshot.imported == false; errors.Is(err, ErrBadPassword) */ }
func TestSync_NotConfigured(t *testing.T) { /* Provider "" -> ErrSyncNotConfigured */ }
func TestSync_PushSavesReturnedGistID(t *testing.T) { /* backend returns "g1"; settings.Sync.GistID=="g1" when Provider=="gist" */ }
```

- [ ] **Step 2: Run** `go test ./internal/core/services/ -run TestSync -v` → FAIL

- [ ] **Step 3: Implement** `Push`/`Pull` as spec; `Pull` order: Download → openBundle → Import (so bad password never touches files).

- [ ] **Step 4: Run** `make test && make build` → PASS

- [ ] **Step 5: Commit**

```bash
git add cmd internal
git commit -m "feat: add sync service with push and pull"
```

### Task 9: Sync screen

**Files:**
- Create: `internal/adapters/ui/sync_form.go`
- Modify: `internal/adapters/ui/handlers.go:40-95` (`handleGlobalKeys`: add `case 'y': t.handleSync()`)
- Modify: `internal/adapters/ui/status_bar.go:23` (append `• [white]y[-] Sync`)
- Modify: `README.md` (Features: sync + embedded keys)

**Interfaces:**
- Consumes: `t.syncService` (Task 8).
- Produces: `func (t *tui) handleSync()`, `func (t *tui) showSyncForm(s domain.Settings)`.

- [ ] **Step 1: Implement the form** per spec (provider dropdown, file path, gist id, token as `AddPasswordField`, password `AddPasswordField`, buttons Push / Pull / Save settings / Cancel, footer with last push/pull). `Pull` → confirm modal with the exact spec text → goroutine → `QueueUpdateDraw` → `refreshServerList()` + status. Errors shown via `showStatusTempColor(..., "#FF6B6B")`.

- [ ] **Step 2: Manual verification**

Run: `go run ./cmd` → `y` → provider `file`, path `/tmp/lazyssh-sync.enc`, password `x` → Push → status "Pushed". Edit a server, Pull → confirm → list shows the pre-edit state; `ls ~/.ssh/config*` shows a new backup.
Expected: both status messages green; `file /tmp/lazyssh-sync.enc` is JSON with no `Host` plaintext.

- [ ] **Step 3: Run** `make build && make test` → PASS

- [ ] **Step 4: Commit**

```bash
git add internal/adapters/ui README.md
git commit -m "feat: add sync screen for push and pull"
```

---

# Phase 3: SFTP file manager

### Task 10: SFTP session over system ssh

**Files:**
- Modify: `go.mod` (`go get github.com/pkg/sftp@v1.13.12`)
- Modify: `internal/core/ports/services.go` (append `SFTPSession`, `SFTPService`)
- Create: `internal/core/services/sftp_service.go`, `internal/core/services/sftp_service_test.go`
- Modify: `cmd/main.go`, `internal/adapters/ui/tui.go` (`NewTUI` gains `sftp ports.SFTPService`)

**Interfaces:**
- Produces:
  - `ports.SFTPSession{ Client() *sftp.Client; Close() error }`
  - `ports.SFTPService{ Connect(alias string) (SFTPSession, error) }`
  - `services.NewSFTPService(logger) ports.SFTPService` → runs `exec.Command("ssh", "-s", "sftp", alias)`
  - unexported `newSFTPServiceWithCmd(logger, func(alias string) *exec.Cmd)`

- [ ] **Step 1: Write the failing tests** (helper-process idiom: `TestHelperProcess` serves `sftp.NewServer` over stdin/stdout when `GO_WANT_SFTP_HELPER=1`; a second mode `GO_WANT_SFTP_HELPER=fail` exits 255 immediately)

```go
func TestConnect_ListsDirectoryAndCloses(t *testing.T) {
	svc := newSFTPServiceWithCmd(zap.NewNop().Sugar(), helperCmd("1"))
	sess, err := svc.Connect("any"); if err != nil { t.Fatal(err) }
	dir := t.TempDir(); os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o600)
	entries, err := sess.Client().ReadDir(dir); if err != nil || len(entries) != 1 { t.Fatal(err, entries) }
	if err := sess.Close(); err != nil { t.Fatal(err) }
}
func TestConnect_ProcessExitReturnsError(t *testing.T) {
	// helperCmd("fail"); Connect must return err within 5s (use a done channel + time.After)
}
```

- [ ] **Step 2: Run** `go test ./internal/core/services/ -run 'TestConnect' -v` → FAIL

- [ ] **Step 3: Implement**: `StdinPipe`, `StdoutPipe`, stderr captured into a bounded buffer (last 4 KiB) for the error message; `cmd.Start`; `sftp.NewClientPipe(stdout, stdin)`; on error `cmd.Process.Kill()` + `cmd.Wait()`. `Close`: `client.Close()`, `stdin.Close()`, `Process.Kill()`, `Wait()`. Add `// ponytail: subprocess transport via system ssh; swap for x/crypto/ssh only if in-process is required.`

- [ ] **Step 4: Run** `make test && make build` → PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum cmd internal
git commit -m "feat: open sftp sessions through the system ssh client"
```

### Task 11: Transfer operations

**Files:**
- Create: `internal/core/services/sftp_transfer.go`, `internal/core/services/sftp_transfer_test.go`

**Interfaces:**
- Produces (package `services`, exported for the UI):
  - `type TransferProgress struct{ File string; Done, Total int64; FileIndex, FileCount int }`
  - `func UploadPath(c *sftp.Client, localPath, remoteDir string, progress func(TransferProgress)) error`
  - `func DownloadPath(c *sftp.Client, remotePath, localDir string, progress func(TransferProgress)) error`
  - `func RemoveRemote(c *sftp.Client, path string) error` (recursive)
  - Files and directories; destination name = `filepath.Base(src)` inside the destination dir; mode preserved best-effort; 1 MiB buffer.

- [ ] **Step 1: Write the failing tests** (in-process server: `net.Pipe()`, `sftp.NewServer(serverConn)` in a goroutine, `sftp.NewClientPipe(clientConn, clientConn)`)

```go
func TestUploadPath_FileWithProgress(t *testing.T) { /* 3 MiB file; dest content equal; last progress Done==Total; mode 0640 preserved */ }
func TestUploadPath_DirectoryRecursive(t *testing.T) { /* a/b/c.txt -> dest/a/b/c.txt; FileCount==2 for two files */ }
func TestDownloadPath_FileAndDirectory(t *testing.T) {}
func TestRemoveRemote_Recursive(t *testing.T) {}
```

- [ ] **Step 2: Run** `go test ./internal/core/services/ -run 'Upload|Download|RemoveRemote' -v` → FAIL

- [ ] **Step 3: Implement** with `filepath.WalkDir` (upload) and `c.Walk` (download). Progress callback wrapped in an `io.Writer` counter.

- [ ] **Step 4: Run** `make test` → PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/services/sftp_transfer.go internal/core/services/sftp_transfer_test.go
git commit -m "feat: add sftp upload, download and recursive remove"
```

### Task 12: File manager screen

**Files:**
- Create: `internal/adapters/ui/sftp_pane.go`, `internal/adapters/ui/sftp_pane_test.go`, `internal/adapters/ui/sftp_view.go`
- Modify: `internal/adapters/ui/handlers.go` (`case 'm': t.handleFileManager()`)
- Modify: `internal/adapters/ui/status_bar.go:23` (append `• [white]m[-] Files`)
- Modify: `README.md` (Features: SFTP file manager, key table)

**Interfaces:**
- Consumes: `ports.SFTPService`, `services.UploadPath/DownloadPath/RemoveRemote`, `TransferProgress` (Tasks 10–11).
- Produces:
  - `type fileEntry struct{ Name string; IsDir bool; Size int64; ModTime time.Time }`
  - `func sortEntries(e []fileEntry)` (dirs first, then case-insensitive name)
  - `func formatSize(n int64) string` (`0`, `512`, `1.2K`, `3.4M`, `1.1G`)
  - `type filePane struct{ *tview.Table; title string; cwd string; list func(path string) ([]fileEntry, error) }` with `func newFilePane(title string, list func(string) ([]fileEntry, error)) *filePane`, `func (p *filePane) Load(path string) error`, `func (p *filePane) Selected() (fileEntry, bool)`, `func (p *filePane) ParentDir() string` (uses `path.Dir` for remote, `filepath.Dir` for local; pass a `join func(...string) string` into the pane).
  - `func (t *tui) handleFileManager()`: `app.Suspend(func(){ sess, err = sftpService.Connect(alias) })`, then `showFileManager(alias, sess)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestSortEntries_DirsFirstCaseInsensitive(t *testing.T) { /* [b.txt, A/, a.txt, B/] -> [A/, B/, a.txt, b.txt] */ }
func TestFormatSize(t *testing.T) { /* 0->"0", 512->"512", 1234->"1.2K", 3_500_000->"3.3M", 1_200_000_000->"1.1G" */ }
func TestFilePane_LoadRendersParentRow(t *testing.T) { /* fake list returns 2 entries; GetRowCount()==3+header; row1 == ".." */ }
```

- [ ] **Step 2: Run** `go test ./internal/adapters/ui/ -run 'SortEntries|FormatSize|FilePane' -v` → FAIL

- [ ] **Step 3: Implement** `sftp_view.go` per spec key table. Layout: `Flex` row with the two panes, a 1-line status `TextView`, root set with `app.SetRoot`. Transfers run in a goroutine; progress via `QueueUpdateDraw` on the status line; overwrite check via `client.Stat`/`os.Stat` → confirm modal before starting. `q`/`Esc` → `sess.Close()` → `returnToMain()`. Delete uses a modal with the name in it; dirs removed recursively (`RemoveRemote` or `os.RemoveAll`).

- [ ] **Step 4: Manual verification**

Run: `go run ./cmd` → select a reachable server → `m` → both panes list; `u` on a local file → appears remote; `d` → appears local; `n` creates a dir; `D` removes it; `q` returns to main with no leftover `ssh` process (`pgrep -f 'ssh -s sftp'` empty).

- [ ] **Step 5: Run** `make build && make test` → PASS

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/ui README.md
git commit -m "feat: add sftp dual-pane file manager"
```

---

## Self-review notes

- Spec coverage: Phase 1 → Tasks 1–3; settings → 4; bundle/snapshot → 5; crypto → 6; backends → 7; service → 8; sync UI → 9; SFTP connect → 10; transfers → 11; UI → 12. README updates folded into 9 and 12.
- Type consistency: `PrivateKey` (1,2,3); `SyncBundle`/`SnapshotRepository` (5,8); `SyncBackend.Upload` returns `(string, error)` (7,8); `TransferProgress`, `UploadPath`, `DownloadPath`, `RemoveRemote` (11,12); `NewTUI` signature changes in 8 and again in 10.
- Review Focus items 1–5 are pinned to Tasks 1, 8, 7, 10, 2 respectively.
