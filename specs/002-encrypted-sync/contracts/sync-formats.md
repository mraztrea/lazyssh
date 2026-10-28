# Contracts: Settings File and Encrypted Sync

## Files

| Path | Mode | Content |
|---|---|---|
| `~/.lazyssh/settings.json` | 0600 (dir 0700) | `{"sync":{...}}`, see data-model.md |
| `<file_path>` (provider `file`) | 0600 | envelope JSON |
| `~/.ssh/config` after Pull | 0600 | bundle `ssh_config`; previous content in `config-<ms>-lazyssh.backup` |
| `~/.lazyssh/metadata.json` after Pull | 0600 | bundle `metadata` |

## Envelope (wire format, both providers)

```json
{ "v": 1, "kdf": "pbkdf2-sha256", "iter": 600000, "salt": "<base64 16B>", "nonce": "<base64 12B>", "data": "<base64 AES-256-GCM(gzip(json bundle))>" }
```

## GitHub Gist API (provider `gist`)

Headers on API calls: `Authorization: Bearer <token>`, `Accept: application/vnd.github+json`.
Client timeout 30 s.

| Operation | Request | Success | Used response fields |
|---|---|---|---|
| first push (no id) | `POST /gists` `{"description":"lazyssh sync","public":false,"files":{"lazyssh-sync.enc":{"content":"<envelope>"}}}` | 201 | `id` → `settings.sync.gist_id` |
| later push | `PATCH /gists/{id}` `{"files":{"lazyssh-sync.enc":{"content":"<envelope>"}}}` | 200 | `id` |
| pull | `GET /gists/{id}` | 200 | `files["lazyssh-sync.enc"].content`, `.truncated`, `.raw_url` |
| pull, truncated | `GET <raw_url>` (no Authorization) | 200 | body = envelope |

Errors: non-2xx → `github: <status>: <message>`; pull with empty id or without the file →
`ErrNoRemote`.

## Errors (user-visible text)

| Name | Package | Text |
|---|---|---|
| `ErrPasswordRequired` | services | `password is required` |
| `ErrBadPassword` | services | `wrong password or corrupted sync data` |
| `ErrSyncNotConfigured` | services | `sync is not configured: choose provider file or gist` |
| `ErrNoRemote` | ports | `no remote sync data found` |

## UI

| Element | Contract |
|---|---|
| Global key | `y` opens Sync form; status bar lists `y Sync` |
| Form fields | `Provider:` (`file`, `gist`), `File path:`, `Gist ID:`, `GitHub token:` (masked), `Password:` (masked, never saved) |
| Buttons | `Push`, `Pull`, `Save settings`, `Cancel` (Esc = Cancel) |
| Footer | `Last push: <RFC3339 local or never> / Last pull: <… or never>` |
| Pull confirm | `Pull will REPLACE ~/.ssh/config and ~/.lazyssh/metadata.json (backup is taken). Continue?` |
| Status | `Pushing…` / `Pulling…`, then `Pushed (<provider>)` / `Pulled (<provider>)` green or `Push failed: <err>` / `Pull failed: <err>` red |
