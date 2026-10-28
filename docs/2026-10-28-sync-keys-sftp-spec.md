# Spec: Config Sync, Embedded Private Keys, SFTP File Manager

Date: 2026-10-28. Target repo: `github.com/Adembc/lazyssh` (Go 1.24, tview TUI, hexagonal layout `domain → ports → services → adapters`).

Three features, shipped as three phases. Each phase is independently usable.

| Phase | Feature | Depends on |
|---|---|---|
| 1 | Embedded private keys (key text stored in `~/.lazyssh/metadata.json`) | nothing |
| 2 | Settings file + encrypted sync (local file or GitHub Gist, electerm-style) | Phase 1 (keys ride along in metadata) |
| 3 | SFTP file manager (dual pane, upload/download, mkdir/rename/delete) | nothing (ships after 2 for ordering only) |

## Assumptions made without the user (change the spec, not the code, if wrong)

1. **Sync is manual push/pull, no merge.** electerm also does whole-bundle upload/download. Pull replaces local files after taking a backup.
2. **Sync bundle is always encrypted** with a user password (AES-256-GCM, PBKDF2-SHA256). Private keys live in the bundle, so there is no unencrypted mode.
3. **Password is not stored.** It is asked on every push/pull. The GitHub token is stored in `~/.lazyssh/settings.json` (mode 0600), same as electerm.
4. **SFTP goes through the system `ssh` binary** (`ssh -s sftp <alias>`) piped into `pkg/sftp`. This reuses the user's full SSH config (ProxyJump, agent, ControlMaster, passphrase prompts). No in-process SSH client.
5. **Embedded keys are materialised to disk** at `~/.lazyssh/keys/<alias>` (0600) and referenced by `IdentityFile`. OpenSSH needs a file path; this keeps `ssh <alias>` and SFTP working unchanged.

## Phase 1: Embedded private keys

### Data model

- `domain.Server` gains `PrivateKey string` (PEM text, may be passphrase-protected).
- `ssh_config_file.ServerMetadata` gains `PrivateKey string \`json:"private_key,omitempty"\``.
- Key file path: `filepath.Join(filepath.Dir(metadataPath), "keys", alias)`. Alias is already restricted to `^[A-Za-z0-9_.-]+$`, so it is filename-safe. Reject alias `.` and `..` in `validateServer`.

### Behaviour

| Event | Effect |
|---|---|
| Add/Update server with `PrivateKey != ""` | Write key file (0600, dir 0700). Ensure `IdentityFiles` contains the key path (prepend if missing). Persist `private_key` in metadata. |
| Update server with `PrivateKey == ""` and metadata previously had one | Delete key file. Remove key path from `IdentityFiles`. Clear `private_key`. |
| Alias rename with embedded key | Remove old key file, write new one, replace path in `IdentityFiles`. |
| Delete server | Delete key file if present. |
| `ListServers` | For every server whose metadata has `private_key` and whose key file is missing or differs, rewrite the file. This is what makes keys appear after a sync pull on another machine. |

### Validation (`services.validateServer`)

- `PrivateKey` non-empty must parse with `ssh.ParseRawPrivateKey`. A `*ssh.PassphraseMissingError` is accepted (encrypted key). Any other error → `"private key is not a valid OpenSSH/PEM private key"`.
- Trailing newline is normalised: stored text always ends with exactly one `\n`.

### UI

- Authentication tab, new section `▶ Embedded Private Key (synced)` placed first in the tab:
  - `PrivateKey:` multi-line `TextArea` (width 60, height 8). Paste the key here.
  - `Import key from file:` `InputField` with the existing SSH-key autocomplete. On Save, if `PrivateKey` text is empty and this path is non-empty, the file is read and its content becomes `PrivateKey`. `~` is expanded.
- Server details pane: `Key:` line shows `(embedded)` suffix when `PrivateKey != ""`. The key text is never rendered.
- Field help entry for `PrivateKey` explaining that the key is stored in `~/.lazyssh/metadata.json` and included in sync.

## Phase 2: Settings + encrypted sync

### Settings file

`~/.lazyssh/settings.json`, mode 0600:

```json
{
  "sync": {
    "provider": "file",            // "file" | "gist"
    "file_path": "~/Dropbox/lazyssh-sync.enc",
    "gist_id": "",
    "gist_token": "",
    "last_push_at": "2026-10-28T10:00:00Z",
    "last_pull_at": ""
  }
}
```

- `domain.Settings{ Sync SyncSettings }`, `domain.SyncSettings{ Provider, FilePath, GistID, GistToken string; LastPushAt, LastPullAt time.Time }`.
- `ports.SettingsRepository{ Load() (domain.Settings, error); Save(domain.Settings) error }`. Missing file → zero value, no error.

### Bundle

`domain.SyncBundle`:

```go
type SyncBundle struct {
    Version    int             `json:"version"`      // 1
    ExportedAt time.Time       `json:"exported_at"`
    Hostname   string          `json:"hostname"`     // os.Hostname() of the pusher, informational
    SSHConfig  string          `json:"ssh_config"`   // raw text of ~/.ssh/config
    Metadata   json.RawMessage `json:"metadata"`     // raw ~/.lazyssh/metadata.json (includes private keys)
}
```

Settings are **not** in the bundle (token and file path are machine-local, same as electerm).

`ports.SnapshotRepository{ Export() (domain.SyncBundle, error); Import(domain.SyncBundle) error }` implemented in `ssh_config_file`:
- `Export` reads both files. Missing `~/.ssh/config` → empty string. Missing metadata → `{}`.
- `Import` backs up `~/.ssh/config` (existing `createOriginalBackupIfNeeded` + `createBackup`), then atomically writes config and metadata (temp file + rename, 0600).

### Encryption (`services/sync_crypto.go`)

Envelope on the wire, JSON:

```json
{ "v": 1, "kdf": "pbkdf2-sha256", "iter": 600000, "salt": "<base64 16B>", "nonce": "<base64 12B>", "data": "<base64 AES-256-GCM(gzip(json bundle))>" }
```

- `sealBundle(b domain.SyncBundle, password string) ([]byte, error)`
- `openBundle(blob []byte, password string) (domain.SyncBundle, error)`; wrong password → `ErrBadPassword`.
- Empty password is rejected: `ErrPasswordRequired`.

### Backends

`ports.SyncBackend{ Upload(ctx context.Context, blob []byte) (remoteID string, err error); Download(ctx context.Context) ([]byte, error) }`

`remoteID` is the identifier the backend wants persisted (gist id; the file path for the file backend). The service writes it back to `Settings.Sync.GistID` when the provider is `gist`.

- **file** (`adapters/sync/file_backend.go`): writes `FilePath` atomically (0600). Download of missing file → `ErrNoRemote`.
- **gist** (`adapters/sync/gist_backend.go`): stdlib `net/http` only. Gist file name is fixed `lazyssh-sync.enc`. If `GistID` is empty on first upload, `POST /gists` (secret gist, description `lazyssh sync`) and the returned id is saved into settings. Otherwise `PATCH /gists/{id}`. Download: `GET /gists/{id}`, read `files["lazyssh-sync.enc"].content`; if `truncated` is true, fetch `raw_url`. HTTP timeout 30s. Non-2xx → error including status and the `message` field.

### Service

`ports.SyncService`:

```go
type SyncService interface {
    Settings() (domain.Settings, error)
    SaveSettings(domain.Settings) error
    Push(ctx context.Context, password string) error
    Pull(ctx context.Context, password string) error
}
```

- `Push`: Export → seal → backend.Upload → set `LastPushAt` → SaveSettings.
- `Pull`: backend.Download → open → Import → set `LastPullAt` → SaveSettings.
- Backend is chosen from `Settings.Sync.Provider`; unknown/empty → `ErrSyncNotConfigured`.

### UI

- Global key `y` opens **Sync** screen (a `tview.Form`):
  - `Provider:` dropdown `file | gist`
  - `File path:` input (shown for both, used by `file`)
  - `Gist ID:` input, `GitHub token:` password field
  - `Password:` password field (never persisted)
  - Buttons: `Push`, `Pull`, `Save settings`, `Cancel`
  - Footer text shows `Last push: … / Last pull: …`.
- `Pull` asks a confirm modal: `"Pull will REPLACE ~/.ssh/config and ~/.lazyssh/metadata.json (backup is taken). Continue?"`.
- Push/Pull run in a goroutine; result shown in status bar; server list refreshed after Pull.
- Status bar gains `y Sync`.

## Phase 3: SFTP file manager

### Connection (`services/sftp_service.go`)

```go
type SFTPSession interface {
    Client() *sftp.Client
    Close() error
}
type SFTPService interface {
    Connect(alias string) (SFTPSession, error)
}
```

- `Connect` runs `ssh -s sftp <alias>` via `exec.Command`, stdin/stdout piped, stderr → log. Returns `sftp.NewClientPipe(stdout, stdin)`. `Close` closes the client, then kills the process.
- The TUI calls `Connect` inside `app.Suspend` so OpenSSH can prompt for passphrase/password on the tty. Failure message includes the last stderr line.
- `ponytail:` subprocess transport. Upgrade to `golang.org/x/crypto/ssh` only if an in-process client is ever required.

### UI (`adapters/ui/sftp_view.go`, `sftp_pane.go`)

Global key `m` on a selected server opens the file manager. Layout:

```
┌ Local: /home/me/projects ───────┐┌ Remote: /var/www (alias) ────────┐
│ ../                             ││ ../                              │
│ dir/                            ││ app/                             │
│ file.txt           1.2K  10-28  ││ index.html        4.1K  10-27    │
└─────────────────────────────────┘└──────────────────────────────────┘
 status / transfer progress line
```

Each pane is a `tview.Table` (name, size, mtime). Directories first, then files, both case-insensitive by name.

| Key | Action |
|---|---|
| `Tab` | Switch pane |
| `Enter` | Enter directory (`..` goes up) |
| `Backspace` / `h` | Go to parent |
| `j` / `k` / arrows | Move |
| `u` | Upload selected local file/dir to remote cwd |
| `d` | Download selected remote file/dir to local cwd |
| `n` | New directory in active pane (prompt) |
| `r` | Rename selected (prompt) |
| `D` | Delete selected (confirm modal); directories deleted recursively |
| `R` | Refresh both panes |
| `g` | Go to path (prompt), active pane |
| `q` / `Esc` | Close session, back to main |

Transfers:
- One transfer at a time, run in a goroutine. Progress line: `Uploading file.txt  45%  2.1 MB/s`. Directories are walked and transferred file by file; progress shows `n/total files`.
- Existing destination file → confirm modal `Overwrite <name>?`.
- Copy uses 1 MiB buffer and `io.Copy`. File mode is preserved (`chmod` after write, best effort).
- Transfer error → red status, panes refreshed; partial file left in place (user can retry).

Start directories: local = `os.UserHomeDir()`, remote = `client.Getwd()`.

## Global constraints

- Go 1.24+, `make build` (runs lint) and `make test` must pass.
- New deps allowed: `github.com/pkg/sftp`, `golang.org/x/crypto` (transitive of sftp, used directly for key validation). No GitHub SDK, no extra crypto libs.
- Files written under `~/.lazyssh` are 0600, directories 0700.
- No private key text ever appears in logs, status bar, or details pane.
- Keep the Apache license header on new files.
- Commit messages follow repo convention: `feat:`, `fix:`, `improve:`, `test:`.

## Out of scope (YAGNI, revisit if asked)

- Automatic sync on change, conflict merge, more than one remote.
- Multiple parallel transfers, resume, drag and drop.
- Storing the encryption password.
- In-process SSH (no system `ssh` binary).
