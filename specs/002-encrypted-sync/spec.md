# Feature Specification: Settings File and Encrypted Sync

**Feature Branch**: `002-encrypted-sync`

**Created**: 2026-10-28

**Status**: Implemented

**Input**: User description: "Settings file and encrypted sync: store app settings in ~/.lazyssh/settings.json; push/pull an encrypted bundle (ssh config + metadata incl. embedded keys) to a local file path or a GitHub Gist, electerm-style, with a user password that is never stored."

**Source document**: `docs/2026-10-28-sync-keys-sftp-spec.md`, section "Phase 2: Settings + encrypted sync" (plus "Assumptions", "Global constraints", "Out of scope").

## Clarifications

### Session 2026-10-28

Answered non-interactively from the source spec and `docs/2026-10-28-sync-keys-sftp-plan.md`.

- Q: After pressing Push or Pull, does the Sync screen stay open while the operation runs? → A: No; the app returns to the main screen at once, shows `Pushing…`/`Pulling…`, and the result replaces it in the status bar (source spec: "result shown in status bar"; the status bar is only visible on the main screen).
- Q: Is an empty password rejected before anything is exported or downloaded? → A: Yes; Push and Pull fail with `ErrPasswordRequired` before touching the backend, and the encryption layer also rejects it (source spec "Empty password is rejected").
- Q: What does Pull do with provider `gist` and an empty gist id? → A: Fails with `ErrNoRemote` without any HTTP request (nothing was ever pushed from this configuration).
- Q: Which form values does Push/Pull persist before running? → A: Provider, file path, gist id and token from the form are saved to settings first; the password never is. After a successful gist push the returned id is saved and shown next time the form opens.
- Q: What are the success messages? → A: `Pushed (<provider>)` and `Pulled (<provider>)` in green; errors as `Push failed: <err>` / `Pull failed: <err>` in red (plan Task 9: status "Pushed").

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Push my servers to a sync location (Priority: P1)

A user opens the Sync screen (key `y`), picks provider `file`, enters a path such as
`~/Dropbox/lazyssh-sync.enc` and a password, and presses `Push`. lazyssh packs `~/.ssh/config`
and `~/.lazyssh/metadata.json` (including embedded private keys) into one bundle, encrypts it with
the password, and writes it to the path. The status bar confirms the push and the screen footer
shows the new `Last push:` time.

**Why this priority**: Without push there is nothing to pull; it is the smallest useful slice
(an encrypted, portable backup).

**Independent Test**: Push to a temp path, then confirm the file exists with mode 0600, is JSON
with the envelope fields, and contains no `Host` plaintext and no key text.

**Acceptance Scenarios**:

1. **Given** provider `file`, a path and a non-empty password, **When** the user presses `Push`,
   **Then** the encrypted envelope is written atomically to the path (mode 0600), the status bar
   shows success, and `last_push_at` is saved in `~/.lazyssh/settings.json`.
2. **Given** an empty password, **When** the user presses `Push`, **Then** nothing is uploaded and
   the status bar shows `Push failed: password is required` (`ErrPasswordRequired`).
3. **Given** no provider selected/saved, **When** the user pushes, **Then** the error is
   `ErrSyncNotConfigured` and nothing is written.

---

### User Story 2 - Pull my servers on another machine (Priority: P1)

On a second machine the user configures the same location, enters the same password and presses
`Pull`. A confirm modal warns `Pull will REPLACE ~/.ssh/config and ~/.lazyssh/metadata.json
(backup is taken). Continue?`. On confirm, lazyssh downloads and decrypts the bundle, backs up the
current SSH config, replaces both files, refreshes the server list, and embedded keys reappear in
`~/.lazyssh/keys/` on that refresh.

**Why this priority**: Pull is the other half of sync; together with US1 it is the feature.

**Independent Test**: Push from state A, edit a server (state B), pull: the list shows state A
again, and a new `*lazyssh.backup` file of state B exists next to `~/.ssh/config`.

**Acceptance Scenarios**:

1. **Given** a pushed bundle, **When** the user pulls with the right password and confirms,
   **Then** `~/.ssh/config` and `~/.lazyssh/metadata.json` equal the pushed content, a backup
   ending in `lazyssh.backup` of the previous config exists, the server list is refreshed, and
   `last_pull_at` is saved.
2. **Given** a pushed bundle, **When** the user pulls with a wrong password, **Then** the error is
   `ErrBadPassword` and neither `~/.ssh/config` nor `metadata.json` is touched (no backup, no
   write).
3. **Given** provider `file` and a path that does not exist, **When** the user pulls, **Then** the
   error is `ErrNoRemote` (`no remote sync data found`) and local files are untouched.
4. **Given** the confirm modal, **When** the user cancels, **Then** nothing is downloaded or
   changed.

---

### User Story 3 - Sync through a GitHub Gist (Priority: P2)

The user picks provider `gist`, pastes a GitHub token (with gist scope), leaves `Gist ID` empty,
and pushes. lazyssh creates a secret gist (description `lazyssh sync`) holding one file
`lazyssh-sync.enc` and saves the new gist id into settings. Later pushes update that gist; pulls on
any machine with the same gist id, token and password restore the servers.

**Why this priority**: Remote sync without a shared folder; builds on US1/US2 with another backend.

**Independent Test**: Against a fake GitHub HTTP server: first push sends `POST /gists`, the
returned id is saved; second push sends `PATCH /gists/{id}`; pull reads the file content, following
`raw_url` when the response marks the file `truncated`.

**Acceptance Scenarios**:

1. **Given** provider `gist`, empty gist id, a token, **When** the user pushes, **Then** a secret
   gist is created and its id is written to `gist_id` in settings and shown in the form.
2. **Given** a saved gist id, **When** the user pushes, **Then** the existing gist is updated (not
   a new one created).
3. **Given** a large bundle that GitHub reports as `truncated: true`, **When** the user pulls,
   **Then** the full content is fetched from `raw_url` and decrypts correctly.
4. **Given** a bad token, **When** the user pushes or pulls, **Then** the error shows the HTTP
   status and GitHub's `message` (e.g. `Bad credentials`) and never the token.

---

### User Story 4 - Keep sync settings between runs (Priority: P3)

The user fills in provider, file path, gist id and token and presses `Save settings`. Next time the
Sync screen opens the fields are prefilled (the token is masked); the password field is always
empty.

**Why this priority**: Convenience; push/pull work without it because they also save the form.

**Independent Test**: Save settings, restart, open Sync: fields are prefilled from
`~/.lazyssh/settings.json` (mode 0600); the password is not in that file.

**Acceptance Scenarios**:

1. **Given** filled fields, **When** the user presses `Save settings`, **Then** `settings.json` is
   written with mode 0600 (directory 0700) and contains no password.
2. **Given** no `settings.json`, **When** the Sync screen opens, **Then** all fields are empty and
   no error is shown.

---

### Edge Cases

- Missing `~/.ssh/config` on push → bundle carries an empty config; missing/empty
  `metadata.json` → bundle carries `{}`.
- Missing `~/.ssh/config` on pull → no backup is made (nothing to back up); files are still written.
- Bundle with a `version` other than 1 → import is rejected, local files untouched.
- Tampered or truncated blob → decryption fails with `ErrBadPassword`; local files untouched.
- Unknown provider string in settings → `ErrSyncNotConfigured`.
- `file_path` starting with `~` → expanded to the home directory.
- Push/Pull runs in the background; the UI returns to the main screen, stays responsive, and the
  result arrives in the status bar.
- Pull with provider `gist` and an empty gist id → `ErrNoRemote`, no HTTP request.

## Requirements *(mandatory)*

### Settings

- **FR-001**: App settings MUST be stored in `~/.lazyssh/settings.json`, mode 0600 (directory
  0700), with exactly this shape (all fields omitted when empty/zero):

```json
{
  "sync": {
    "provider": "file",
    "file_path": "~/Dropbox/lazyssh-sync.enc",
    "gist_id": "",
    "gist_token": "",
    "last_push_at": "2026-10-28T10:00:00Z",
    "last_pull_at": ""
  }
}
```

  `provider` is `"file"` or `"gist"`. Times are RFC 3339. The example shows every key; on disk,
  empty strings and unset times are omitted rather than written as `""`.
- **FR-002**: A missing settings file MUST load as empty settings without error.
- **FR-003**: The sync password MUST NOT be stored anywhere (not in settings, not in logs).

### Bundle

- **FR-004**: The sync bundle MUST be this JSON object:

| Field | Meaning |
|---|---|
| `version` | always `1` |
| `exported_at` | time of export |
| `hostname` | host name of the pushing machine, informational |
| `ssh_config` | raw text of `~/.ssh/config` (`""` if missing) |
| `metadata` | raw JSON of `~/.lazyssh/metadata.json`, including `private_key` entries (`{}` if missing or empty) |

- **FR-005**: Settings MUST NOT be part of the bundle (token and file path are machine-local).
- **FR-006**: Importing a bundle MUST reject any `version` other than 1, then back up
  `~/.ssh/config` (one-time original backup plus a timestamped `*lazyssh.backup`, same as edits),
  then atomically replace `~/.ssh/config` and `~/.lazyssh/metadata.json` (0600).

### Encryption

- **FR-007**: The bundle MUST always be encrypted; there is no plaintext mode. The wire format is
  the JSON envelope:

```json
{ "v": 1, "kdf": "pbkdf2-sha256", "iter": 600000, "salt": "<base64 16B>", "nonce": "<base64 12B>", "data": "<base64 AES-256-GCM(gzip(json bundle))>" }
```

  Key = PBKDF2-SHA256(password, salt, 600000 iterations, 32 bytes). Salt (16 B) and nonce (12 B)
  are fresh random values on every push.
- **FR-008**: An empty password MUST be rejected with `ErrPasswordRequired` on push and pull,
  before anything is exported, uploaded or downloaded.
- **FR-009**: A wrong password or modified ciphertext MUST fail with `ErrBadPassword`.

### Backends

- **FR-010**: Provider `file` MUST write the envelope atomically to `file_path` (mode 0600,
  leading `~` expanded); downloading a missing file MUST fail with `ErrNoRemote`.
- **FR-011**: Provider `gist` MUST use the GitHub REST API with header
  `Authorization: Bearer <token>` and `Accept: application/vnd.github+json`, HTTP timeout 30 s:
  - Upload with empty gist id: `POST /gists` creating a secret gist, description `lazyssh sync`,
    one file named `lazyssh-sync.enc` whose content is the envelope; the returned id MUST be saved
    as `gist_id`.
  - Upload with a gist id: `PATCH /gists/{id}` with the same file.
  - Download: `GET /gists/{id}`, content of `files["lazyssh-sync.enc"]`; if that file is marked
    `truncated: true`, the content MUST be fetched from its `raw_url`. A gist without that file →
    `ErrNoRemote`.
  - Any non-2xx response MUST produce an error containing the HTTP status and GitHub's `message`.
- **FR-012**: An empty or unknown provider MUST fail with `ErrSyncNotConfigured`.

### Push / Pull

- **FR-013**: Push order: export bundle → encrypt → upload → set `last_push_at` → save settings.
- **FR-014**: Pull order: download → decrypt → import → set `last_pull_at` → save settings. A
  failure at download or decrypt MUST leave local files untouched.

### UI

- **FR-015**: Global key `y` MUST open a **Sync** form with: `Provider:` dropdown (`file`,
  `gist`), `File path:` input, `Gist ID:` input, `GitHub token:` password field, `Password:`
  password field (never persisted, always empty on open), buttons `Push`, `Pull`,
  `Save settings`, `Cancel`, and a footer `Last push: … / Last pull: …` (`never` when unset).
- **FR-016**: `Pull` MUST first show a confirm modal with the exact text
  `Pull will REPLACE ~/.ssh/config and ~/.lazyssh/metadata.json (backup is taken). Continue?`.
- **FR-017**: Pressing `Push` or `Pull` (after the confirm) MUST save provider, file path, gist id
  and token from the form, return to the main screen showing `Pushing…`/`Pulling…`, and run in the
  background. The result MUST appear in the status bar: `Pushed (<provider>)` / `Pulled
  (<provider>)` in green, `Push failed: <err>` / `Pull failed: <err>` in red. After a successful
  Pull the server list MUST be refreshed.
- **FR-018**: The status bar help MUST include `y Sync`.

### Security

- **FR-019**: The token, the password and bundle contents (config text, metadata, keys) MUST NOT
  appear in logs, the status bar, or error messages. Logs may contain provider, paths, gist ids
  and HTTP status codes.
- **FR-020**: Files written under `~/.lazyssh` MUST be 0600 and directories 0700; the sync file
  written by provider `file` MUST be 0600.

### Key Entities

- **Settings**: machine-local app settings; currently only the `sync` section.
- **Sync settings**: provider, file path, gist id, GitHub token, last push time, last pull time.
- **Sync bundle**: version, export time, pusher host name, SSH config text, metadata JSON.
- **Envelope**: the encrypted wire form of a bundle (`v`, `kdf`, `iter`, `salt`, `nonce`,
  `data`).
- **Sync backend**: a place an envelope is uploaded to and downloaded from (`file` or `gist`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can move their full server list (including embedded keys) to a second machine
  with one Push and one Pull, no manual file copying.
- **SC-002**: 0 bytes of SSH config, metadata or key plaintext appear in the pushed blob (checked
  by test and by manual inspection).
- **SC-003**: A Pull with a wrong password changes 0 local files (pinned by an automated test).
- **SC-004**: Every Pull that replaces an existing `~/.ssh/config` leaves exactly one new
  timestamped backup of the previous content.
- **SC-005**: Bundles of any size round-trip through the gist provider, including when GitHub
  truncates the inline content.
- **SC-006**: The password is never found in `settings.json`, logs, or the status bar.

## Assumptions

- Sync is manual push/pull of the whole bundle, no merge; Pull replaces local files after a backup
  (source Assumption 1, electerm-style).
- The bundle is always encrypted (AES-256-GCM, PBKDF2-SHA256) because it carries private keys
  (source Assumption 2).
- The password is asked on every push/pull and never stored; the GitHub token is stored in
  `settings.json` (0600), same as electerm (source Assumption 3).
- Settings are not synced; each machine keeps its own provider, path and token.
- Embedded keys are restored to `~/.lazyssh/keys/` by the existing Phase 1 behaviour (keys are
  materialised on the next server list load), which the post-pull refresh triggers.
- Metadata is replaced as a whole file (no per-host merge); metadata is not backed up separately
  because the pushed bundle on the remote is itself the copy (only `~/.ssh/config` gets the
  existing backup rotation, as the source spec says).
- Implementation deviations from the source plan (all recorded in research.md / tasks.md):
  - Config write on Import reuses the edit path through `saveRawConfig(content)` (backups + temp
    file + rename) instead of a new `writeRawToFile(path, content)`; behaviour is identical.
  - `last_push_at` / `last_pull_at` use `omitzero` instead of `omitempty` (a zero `time.Time` is
    never omitted by `omitempty`), so unset times are absent rather than `""`.
  - The backend factory is a `backendFor` function in `cmd/main.go` (constitution I), not a
    `services` function; `services.ErrSyncNotConfigured` is still the error it returns.
  - Service tests use in-memory fakes for settings/snapshot/backend instead of the real settings
    repository (constitution I); the real repository has its own tests.
  - Small hardening beyond the plan: provider `file` with an empty path → `ErrSyncNotConfigured`;
    envelope `iter` must be 1..10,000,000 (the value comes from untrusted remote data); the gist
    `raw_url` is fetched without the token; extra tests pin `~` expansion, gist download without
    id/file → `ErrNoRemote`, and empty password reaching neither backend nor import.
  - README key-binding table gains `y` in addition to the Features section.
- Out of scope: automatic sync on change, conflict merge, more than one remote, storing the
  password, SFTP (Phase 3).
