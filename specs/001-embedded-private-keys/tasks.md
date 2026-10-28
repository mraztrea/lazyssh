---

description: "Task list for Embedded Private Keys"
---

# Tasks: Embedded Private Keys

**Input**: Design documents from `/specs/001-embedded-private-keys/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ui-and-files.md

**Tests**: Required. Constitution II and the source plan mandate TDD: write the test, see it fail,
implement, see it pass, commit.

**Organization**: Phases map 1:1 onto source plan (`docs/2026-10-28-sync-keys-sftp-plan.md`)
Tasks 1–3. Each phase ends in exactly one commit with the source plan's message. Story labels show
which spec user stories each task serves.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story from spec.md (US1 paste key, US2 import from file, US3 keep files in
  step, US4 details pane)

## Phase 1: Setup

No setup tasks: Go module, lint and test tooling already exist (`make build` / `make test` green on
the base commit). The new dependency is added inside Phase 2 (T006) because the source plan commits
it with Task 1.

---

## Phase 2: Foundational = Source Task 1 "Domain field, metadata field, validation" (US1)

**Goal**: Servers carry `PrivateKey`; it round-trips through `metadata.json`; the service rejects
invalid keys and `.`/`..` aliases and normalises line endings.

**Independent Test**: `go test ./internal/core/services/ -run 'TestValidateServer|TestNormalize' -v`

- [X] T001 [US1] Write failing tests `TestValidateServer_PrivateKey` (cases: "empty ok", "valid ed25519", "crlf accepted", "garbage" with ed25519 key generated at runtime via `ssh.MarshalPrivateKey`), `TestValidateServer_RejectsDotAliases` (`.`, `..`), `TestNormalizePrivateKey` (`"a\r\nb\r\n\r\n"`→`"a\nb\n"`, `"  \n"`→`""`) in new file internal/core/services/server_service_test.go (Apache header)
- [X] T002 [US1] Run `go test ./internal/core/services/ -run 'TestValidateServer|TestNormalize' -v`; confirm FAIL (compile error: `PrivateKey` / `normalizePrivateKey` undefined)
- [X] T003 [US1] Add `PrivateKey string` after `IdentityFiles` in internal/core/domain/server.go
- [X] T004 [US1] In internal/adapters/data/ssh_config_file/metadata_manager.go add `PrivateKey string \`json:"private_key,omitempty"\`` to `ServerMetadata`; in `updateServer` set `merged.PrivateKey = server.PrivateKey` unconditionally; change `ensureDirectory` mode 0o750 → 0o700 (FR-012)
- [X] T005 [US1] In internal/adapters/data/ssh_config_file/mapper.go `mergeMetadata` set `servers[i].PrivateKey = meta.PrivateKey`
- [X] T006 [US1] `go get golang.org/x/crypto@v0.58.0` (updates go.mod, go.sum; `go` directive → 1.26.0 per research R1)
- [X] T007 [US1] In internal/core/services/server_service.go add `normalizePrivateKey(s string) string` ("converts `\r\n`→`\n`, trims trailing whitespace, appends one `\n`, returns `""` for blank input"); in `validateServer` reject alias `.`/`..` and, for non-empty key, `ssh.ParseRawPrivateKey([]byte(normalizePrivateKey(key)))` accepting `*ssh.PassphraseMissingError` via `errors.As`, else `fmt.Errorf("private key is not a valid OpenSSH/PEM private key")`; in `AddServer`/`UpdateServer` assign `server.PrivateKey = normalizePrivateKey(server.PrivateKey)` before the repository call; replace every `"server", server` / `"server", newServer` log field with `"alias", <x>.Alias` (FR-011)
- [X] T008 [US1] Run `go test ./internal/core/services/ -v` then `make test`; confirm PASS
- [X] T009 [US1] Commit `feat: add embedded private key field with validation` (go.mod go.sum internal/core internal/adapters/data)

**Checkpoint**: key field validated and persisted; no file materialisation yet.

---

## Phase 3: Source Task 2 "Key file materialisation in the repository" (US1, US3) 🎯 MVP

**Goal**: Repository writes/removes/moves `~/.lazyssh/keys/<alias>` and keeps `IdentityFile` in
step; listing repairs missing files.

**Independent Test**: `go test ./internal/adapters/data/ssh_config_file/ -run 'PrivateKey|KeyFile' -v`

- [X] T010 [P] [US3] Write failing tests in new file internal/adapters/data/ssh_config_file/key_store_test.go (Apache header; `t.TempDir()` for config and metadata; `NewRepository(zap.NewNop().Sugar(), cfg, meta)`): `TestAddServer_WithPrivateKey_WritesFileAndIdentityFile` (file `<tmp>/keys/web` exists, mode 0600, content == key; `ListServers("")[0].IdentityFiles[0] == <tmp>/keys/web`; `ListServers("")[0].PrivateKey == key`), `TestUpdateServer_ClearPrivateKey_RemovesFileAndIdentityFile`, `TestUpdateServer_RenameAlias_MovesKeyFile` (web→web2: `keys/web` gone, `keys/web2` exists, `IdentityFiles == [keys/web2]`), `TestDeleteServer_RemovesKeyFile`, `TestListServers_RecreatesMissingKeyFile`
- [X] T011 [US3] Run `go test ./internal/adapters/data/ssh_config_file/ -run 'PrivateKey|KeyFile' -v`; confirm FAIL
- [X] T012 [US3] Create internal/adapters/data/ssh_config_file/key_store.go (Apache header): `type keyStore struct{ dir string }`; `func (k keyStore) path(alias string) string`; `func (k keyStore) write(alias, pem string) error` (MkdirAll 0700, WriteFile 0600); `func (k keyStore) remove(alias string) error` (ignore not-exist); `func (k keyStore) ensure(alias, pem string) error` (write only if missing or content differs); `func withKeyPath(files []string, path string) []string` (prepend if absent); `func withoutKeyPath(files []string, path string) []string`
- [X] T013 [US3] In internal/adapters/data/ssh_config_file/ssh_config_file_repo.go add `keys keyStore` to `Repository`, set `keyStore{dir: filepath.Join(filepath.Dir(metaDataPath), "keys")}` in `NewRepository` and `NewRepositoryWithFS`; wire `AddServer` (write + withKeyPath before `createHostFromServer`), `UpdateServer` (oldPath/newPath cases from plan.md), `DeleteServer` (`keys.remove` after metadata delete), `ListServers` (after `mergeMetadata`, `keys.ensure` per server with key, warn with alias + error only, never fail)
- [X] T014 [US3] Run `make test`; confirm PASS
- [X] T015 [US3] Commit `feat: materialise embedded private keys to ~/.lazyssh/keys` (internal/adapters/data/ssh_config_file)

**Checkpoint**: a key set through the service works with `ssh <alias>`.

---

## Phase 4: Source Task 3 "Form, details, help for embedded key" (US1, US2, US4)

**Goal**: Paste or import the key in the Authentication tab; details pane shows `(embedded)`.

**Independent Test**: `go test ./internal/adapters/ui/ -run TestResolvePrivateKey -v` plus
quickstart.md manual steps 1–6.

- [X] T016 [P] [US2] Write failing `TestResolvePrivateKey` (text wins; empty text reads file → `"KEY\n"`; missing file → error; both empty → `""`) in new file internal/adapters/ui/private_key_test.go (Apache header)
- [X] T017 [US2] Run `go test ./internal/adapters/ui/ -run TestResolvePrivateKey -v`; confirm FAIL (undefined)
- [X] T018 [US2] Create internal/adapters/ui/private_key.go (Apache header) with `func resolvePrivateKey(text, importPath string) (string, error)`: returns `text` if non-blank; else if `importPath` non-blank, expands leading `~` with `os.UserHomeDir`, reads file, returns content; else `""`
- [X] T019 [US1] [US2] In internal/adapters/ui/server_form.go: `ServerFormData` gains `PrivateKey`, `PrivateKeyImportPath`; `getDefaultValues` sets `PrivateKey: sf.original.PrivateKey`; `createAuthenticationForm` starts with `▶ Embedded Private Key (synced)`, `PrivateKey:` TextArea width 60 height 8 (focus → help `PrivateKey`), `Import key from file:` via `addInputFieldWithHelp(form, "Import key from file:", "PrivateKeyImportPath", "", 40, "~/.ssh/id_ed25519")` + `SetAutocompleteFunc(sf.createSSHKeyAutocomplete())`; `getFormData` adds `getTextAreaText` helper (raw text, no trim) and fills both fields; `handleSave` calls `resolvePrivateKey`, on error shows modal `"Import key failed: " + err.Error()` and returns false, else `data.PrivateKey = pk`; `dataToServer` sets `PrivateKey: data.PrivateKey`
- [X] T020 [P] [US4] In internal/adapters/ui/server_details.go append `" (embedded)"` to `serverKey` when `server.PrivateKey != ""`; never render the key text
- [X] T021 [P] [US1] In internal/adapters/ui/field_help.go add `"PrivateKey"` entry next to `"IdentityAgent"`: Category `Authentication`, Description "Private key text stored in ~/.lazyssh/metadata.json and written to ~/.lazyssh/keys/<alias>. Included in sync."
- [X] T022 [US1] Run `make test`; confirm PASS; run quickstart.md manual check under a throwaway `HOME` if a terminal is available
- [X] T023 [US1] Commit `feat: edit embedded private key from the server form` (internal/adapters/ui)

**Checkpoint**: all four user stories usable.

---

## Phase 5: Polish & Cross-Cutting

- [X] T024 Run `make build && make test`; fix lint findings (do not disable linters)
- [X] T025 Grep the diff for any log/status call that formats a `domain.Server` or `PrivateKey` (FR-011)
- [X] T026 Commit speckit artifacts `docs: add speckit artifacts for embedded private keys` (specs/001-embedded-private-keys, .specify/memory/constitution.md)

---

## Dependencies & Execution Order

- Phase 2 (Task 1) blocks Phases 3 and 4 (both consume `domain.Server.PrivateKey`).
- Phase 3 (Task 2) and Phase 4 (Task 3) touch disjoint packages and could run in parallel; they are
  run sequentially to keep one commit per source task in plan order.
- Within a phase: test → see fail → implement → see pass → commit.
- Phase 5 after all.

### Parallel Opportunities

- T010 can be written while Phase 2 implementation is in review (different package).
- T016, T020, T021 touch different files.

## Implementation Strategy

MVP = Phases 2 + 3: keys set through the service are usable by `ssh`. Phase 4 makes it reachable
from the UI. Ship all three together on `feat/sync-keys-sftp`.

## Traceability

| Spec | Tasks |
|---|---|
| FR-001, FR-002 | T003, T004, T005 |
| FR-003, FR-004 | T010–T013 |
| FR-005, FR-006, FR-007 | T001, T007 |
| FR-008 | T016–T019 |
| FR-009 | T020 |
| FR-010 | T021 |
| FR-011 | T007, T013, T020, T025 |
| FR-012 | T004, T012 |
