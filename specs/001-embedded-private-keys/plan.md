# Implementation Plan: Embedded Private Keys

**Branch**: `001-embedded-private-keys` | **Date**: 2026-10-28 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-embedded-private-keys/spec.md`

**Source plan**: `docs/2026-10-28-sync-keys-sftp-plan.md`, Phase 1, Tasks 1–3. Files, signatures and
test names below are copied from it; later phases depend on these exact names.

## Summary

Add `PrivateKey` to `domain.Server` and `ServerMetadata` (`json:"private_key,omitempty"`), validate
and normalise it in `services.validateServer`, materialise it to `~/.lazyssh/keys/<alias>` from the
`ssh_config_file` repository (`keyStore`), and edit it from the Authentication tab of the server form
(`resolvePrivateKey` for paste-or-import). `IdentityFile` points at the materialised file so the
system `ssh` binary works unchanged.

## Technical Context

**Language/Version**: Go 1.24+ source; `go` directive becomes 1.26.0 because the pinned
`golang.org/x/crypto v0.58.0` requires it (see research.md R1)

**Primary Dependencies**: `github.com/rivo/tview` (existing), `golang.org/x/crypto v0.58.0` (new,
`ssh.ParseRawPrivateKey`, `ssh.PassphraseMissingError`)

**Storage**: Files: `~/.ssh/config` (IdentityFile), `~/.lazyssh/metadata.json` (`private_key`),
`~/.lazyssh/keys/<alias>` (0600, dir 0700)

**Testing**: `go test -race ./...` via `make test`; stdlib `testing`, `t.TempDir()`

**Target Platform**: Linux, macOS, Windows terminals (existing targets)

**Project Type**: TUI desktop CLI tool

**Performance Goals**: `ListServers` key reconciliation is one small file read per embedded key; no
target beyond "no visible delay" for < 1000 servers

**Constraints**: Key text never in logs/status/details; 0600 files, 0700 dirs under `~/.lazyssh`

**Scale/Scope**: 3 tasks, ~10 files touched, 2 new source files + 3 new test files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
|---|---|---|
| I. Hexagonal layering | Domain gets one field; validation in `services`; file I/O in `adapters/data`; UI talks only through `ports.ServerService` and `domain.Server`. `resolvePrivateKey` reads a user-chosen file in the UI adapter, same as the existing SSH-key autocomplete which already reads `~/.ssh`. | PASS |
| II. Test-first | Each task starts with the failing test named in the source plan. | PASS |
| III. Minimal deps | Only `golang.org/x/crypto`, named in the spec. | PASS |
| IV. Security | `keyStore.write` uses 0700/0600; existing metadata dir mode 0750 → 0700; service log calls switched from dumping `domain.Server` to `alias` (would otherwise log key text). | PASS (with the two fixes listed) |
| V. Simplicity | `keyStore` is a concrete struct, no interface; no new port; helpers are plain functions. | PASS |

Post-design re-check: PASS, no violations; Complexity Tracking empty.

## Project Structure

### Documentation (this feature)

```text
specs/001-embedded-private-keys/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── ui-and-files.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
go.mod, go.sum                                   # + golang.org/x/crypto v0.58.0 (Task 1)
internal/core/domain/server.go                   # + PrivateKey string (Task 1)
internal/core/services/server_service.go         # validateServer, normalizePrivateKey, Add/Update (Task 1)
internal/core/services/server_service_test.go    # NEW (Task 1)
internal/adapters/data/ssh_config_file/
├── metadata_manager.go                          # ServerMetadata.PrivateKey, updateServer, dir 0700 (Task 1)
├── mapper.go                                    # mergeMetadata copies PrivateKey (Task 1)
├── key_store.go                                 # NEW keyStore + withKeyPath/withoutKeyPath (Task 2)
├── key_store_test.go                            # NEW (Task 2)
└── ssh_config_file_repo.go                      # Repository.keys; List/Add/Update/Delete (Task 2)
internal/adapters/ui/
├── private_key.go                               # NEW resolvePrivateKey (Task 3)
├── private_key_test.go                          # NEW (Task 3)
├── server_form.go                               # TextArea + import field, getFormData, handleSave, dataToServer (Task 3)
├── server_details.go                            # "(embedded)" suffix (Task 3)
└── field_help.go                                # PrivateKey help entry (Task 3)
```

**Structure Decision**: Existing hexagonal layout; no new packages.

## Design per task

### Task 1: Domain field, metadata field, validation

- `domain.Server.PrivateKey string` after `IdentityFiles`.
- `ServerMetadata.PrivateKey string \`json:"private_key,omitempty"\``; `metadataManager.updateServer`
  sets `merged.PrivateKey = server.PrivateKey` unconditionally (clearing works); `mergeMetadata` sets
  `servers[i].PrivateKey = meta.PrivateKey`.
- `normalizePrivateKey(s string) string` (services, unexported): `\r\n`→`\n`, trim trailing
  whitespace, append one `\n`; blank → `""`.
- `validateServer`: reject alias `.`/`..`; non-empty key → `normalizePrivateKey` →
  `ssh.ParseRawPrivateKey`; `errors.As(err, &*ssh.PassphraseMissingError)` accepted; other errors →
  `private key is not a valid OpenSSH/PEM private key`.
- `AddServer`/`UpdateServer` (service): `server.PrivateKey = normalizePrivateKey(server.PrivateKey)`
  before the repository call. Log fields use `alias` instead of the whole server.
- Tests: `TestValidateServer_PrivateKey`, `TestValidateServer_RejectsDotAliases`,
  `TestNormalizePrivateKey` (exact bodies in the source plan).

### Task 2: Key file materialisation in the repository

- `type keyStore struct{ dir string }`, `dir = filepath.Join(filepath.Dir(metaDataPath), "keys")`.
- `func (k keyStore) path(alias string) string`
- `func (k keyStore) write(alias, pem string) error` (MkdirAll 0700, WriteFile 0600)
- `func (k keyStore) remove(alias string) error` (ignore not-exist)
- `func (k keyStore) ensure(alias, pem string) error` (write only if missing or content differs)
- `func withKeyPath(files []string, path string) []string` (prepend if absent)
- `func withoutKeyPath(files []string, path string) []string`
- `Repository` gains `keys keyStore`, set in `NewRepository` and `NewRepositoryWithFS`.
- `AddServer`: key → `write`, `IdentityFiles = withKeyPath(...)` before `createHostFromServer`.
- `UpdateServer`: `oldPath`/`newPath`; key present → `remove(old)` if alias changed, `write(new)`,
  `IdentityFiles = withKeyPath(withoutKeyPath(files, oldPath), newPath)`; key empty → `remove(old)`,
  `IdentityFiles = withoutKeyPath(files, oldPath)`.
- `DeleteServer`: `keys.remove(alias)` after metadata delete.
- `ListServers`: after `mergeMetadata`, `keys.ensure` per server with a key; warn (alias + error
  only) on failure, never fail the list.
- Tests: `TestAddServer_WithPrivateKey_WritesFileAndIdentityFile`,
  `TestUpdateServer_ClearPrivateKey_RemovesFileAndIdentityFile`,
  `TestUpdateServer_RenameAlias_MovesKeyFile`, `TestDeleteServer_RemovesKeyFile`,
  `TestListServers_RecreatesMissingKeyFile`.

### Task 3: Form, details, help for embedded key

- `func resolvePrivateKey(text, importPath string) (string, error)` in `private_key.go`.
- `ServerFormData` gains `PrivateKey string`, `PrivateKeyImportPath string`.
- `createAuthenticationForm`: section `▶ Embedded Private Key (synced)` first; `PrivateKey:`
  TextArea 60×8; `Import key from file:` input (fieldName `PrivateKeyImportPath`, width 40,
  placeholder `~/.ssh/id_ed25519`) with `createSSHKeyAutocomplete()`.
- `getFormData`: `getTextAreaText` helper; `handleSave`: `resolvePrivateKey`, modal
  `Import key failed: <err>` on error; `dataToServer`: `PrivateKey`; `getDefaultValues`:
  `PrivateKey: sf.original.PrivateKey`.
- Details: `serverKey += " (embedded)"` when `PrivateKey != ""`.
- Help: `PrivateKey` entry, Category `Authentication`.
- Test: `TestResolvePrivateKey`.

## Complexity Tracking

No violations.
