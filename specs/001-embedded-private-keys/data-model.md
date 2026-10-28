# Data Model: Embedded Private Keys

## domain.Server (modified)

| Field | Type | Notes |
|---|---|---|
| `PrivateKey` | `string` | PEM text, may be passphrase-protected. Empty = no embedded key. Normalised: LF only, ends with exactly one `\n`. |

Validation (`services.validateServer`):

- Alias not `.` or `..` (plus existing `^[A-Za-z0-9_.-]+$`).
- `PrivateKey` non-empty ⇒ parses with `ssh.ParseRawPrivateKey`, or fails only with
  `*ssh.PassphraseMissingError`; otherwise error `private key is not a valid OpenSSH/PEM private key`.

## ssh_config_file.ServerMetadata (modified)

| Field | JSON | Notes |
|---|---|---|
| `PrivateKey` | `private_key,omitempty` | Source of truth for the key. Set unconditionally on add/update so clearing works. Moves with the metadata entry on alias rename. |

## Key file (derived)

- Path: `<dir(metadata.json)>/keys/<alias>`; directory 0700, file 0600.
- Lifecycle:

| State change | Key file | `IdentityFiles` |
|---|---|---|
| add/update, key set | written | key path prepended if absent |
| update, key cleared | removed (no-op if missing) | key path removed |
| update, alias renamed, key set | old removed, new written | old path removed, new path prepended |
| delete server | removed | n/a (host removed) |
| list, key in metadata, file missing/different | rewritten | unchanged |
