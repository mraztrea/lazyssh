# Contract: User-visible surfaces

## Files

| Path | Mode | Content |
|---|---|---|
| `~/.lazyssh/metadata.json` | 0600 | per alias: `"private_key": "<PEM>\n"` (omitted when empty) |
| `~/.lazyssh/keys/` | 0700 | directory |
| `~/.lazyssh/keys/<alias>` | 0600 | exact `private_key` text |
| `~/.ssh/config` | unchanged | `IdentityFile <abs path to ~/.lazyssh/keys/<alias>>` prepended to the host's IdentityFile lines if not already present |

## Server form, Authentication tab (first section)

```text
▶ Embedded Private Key (synced)
PrivateKey:            [multi-line, 60 wide x 8 high]
Import key from file:  [~/.ssh/id_ed25519        ]  (SSH key autocomplete)
```

- Save with pasted text → text used. Save with empty text + path → file content used (`~` expanded).
- Unreadable path → modal `Import key failed: <error>`, form stays open.
- Invalid key → existing save-error path shows `private key is not a valid OpenSSH/PEM private key`.

## Details pane

`Key: <IdentityFiles joined by ", "> (embedded)` when the server has a key. Key text never shown.

## Field help

`PrivateKey` (Category Authentication): "Private key text stored in ~/.lazyssh/metadata.json and
written to ~/.lazyssh/keys/<alias>. Included in sync."
