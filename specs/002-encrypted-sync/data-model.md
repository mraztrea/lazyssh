# Data Model: Settings File and Encrypted Sync

## Settings (`internal/core/domain/settings.go`)

| Field | Type | JSON | Notes |
|---|---|---|---|
| `Sync` | `SyncSettings` | `sync` | only section today |

## SyncSettings

| Field | Type | JSON | Notes |
|---|---|---|---|
| `Provider` | string | `provider,omitempty` | `"file"` or `"gist"`; anything else → `ErrSyncNotConfigured` |
| `FilePath` | string | `file_path,omitempty` | used by `file`; leading `~` expanded |
| `GistID` | string | `gist_id,omitempty` | set from the upload result when provider is `gist` |
| `GistToken` | string | `gist_token,omitempty` | GitHub token, never logged |
| `LastPushAt` | time.Time | `last_push_at,omitzero` | set after successful Push |
| `LastPullAt` | time.Time | `last_pull_at,omitzero` | set after successful Pull |

Stored at `~/.lazyssh/settings.json`, 0600, dir 0700. Missing file = zero value.

## SyncBundle (`internal/core/domain/sync.go`)

| Field | Type | JSON | Notes |
|---|---|---|---|
| `Version` | int | `version` | `SyncBundleVersion = 1`; Import rejects others |
| `ExportedAt` | time.Time | `exported_at` | |
| `Hostname` | string | `hostname` | informational |
| `SSHConfig` | string | `ssh_config` | raw `~/.ssh/config`, `""` if missing |
| `Metadata` | json.RawMessage | `metadata` | raw `metadata.json` incl. `private_key`, `{}` if missing/empty |

## Envelope (unexported, `services/sync_crypto.go`)

| Field | JSON | Value |
|---|---|---|
| version | `v` | `1` |
| kdf | `kdf` | `"pbkdf2-sha256"` |
| iterations | `iter` | `600000` |
| salt | `salt` | base64, 16 bytes |
| nonce | `nonce` | base64, 12 bytes |
| ciphertext | `data` | base64 AES-256-GCM(gzip(JSON SyncBundle)) |

## Lifecycle

```text
Push: Settings.Load → backendFor → Export → seal → Upload → (gist: GistID=id) → LastPushAt → Settings.Save
Pull: Settings.Load → backendFor → Download → open → Import(backup + replace) → LastPullAt → Settings.Save
```

A failure at any step stops the chain; Pull failures before Import leave local files untouched.
