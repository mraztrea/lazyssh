# Feature Specification: Embedded Private Keys

**Feature Branch**: `001-embedded-private-keys`

**Created**: 2026-10-28

**Status**: Implemented

**Input**: User description: "Embedded private keys: store a private key as text per server in ~/.lazyssh/metadata.json, materialise it to ~/.lazyssh/keys/<alias> (0600) and reference it via IdentityFile so system ssh works; edit it from the server form (paste or import from file)."

**Source document**: `docs/2026-10-28-sync-keys-sftp-spec.md`, section "Phase 1: Embedded private keys".

## Clarifications

### Session 2026-10-28

Answered non-interactively from the source spec and `docs/2026-10-28-sync-keys-sftp-plan.md`.

- Q: When a server is saved with an empty key and never had one, does lazyssh still try to delete a key file and strip the key path from `IdentityFile`? → A: Yes, unconditionally; removing a missing file is a no-op and the path is only stripped if present (plan Task 2: "key empty → remove(old), IdentityFiles = withoutKeyPath(files, oldPath)").
- Q: Is the `Import key from file:` path stored anywhere after saving? → A: No; it is a one-shot input, always empty when the form opens, and only its file content is stored (plan Task 3: default value `""`).
- Q: Does a key imported from a file go through the same normalisation and validation as a pasted key? → A: Yes; the file content becomes `PrivateKey` before the server is validated, so FR-006/FR-007 apply identically.
- Q: How do log lines identify a server once servers carry key text? → A: By alias only; log calls must not dump the whole server record (constitution IV, FR-011).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Paste a private key into a server entry (Priority: P1)

A user adding or editing a server opens the Authentication tab, pastes the private key text into the
`PrivateKey:` box, and saves. lazyssh keeps the key with the server's lazyssh data, writes it to a
private key file that only the user can read, and points the server's `IdentityFile` at it, so
`ssh <alias>` from any terminal authenticates with that key.

**Why this priority**: This is the feature. Without it, keys cannot travel with the server list
(Phase 2 sync depends on it).

**Independent Test**: Add a server with a pasted key, save, then confirm `~/.lazyssh/keys/<alias>`
exists with mode 0600 and the same text, the SSH config entry lists that path as `IdentityFile`, and
reopening the form shows the key text.

**Acceptance Scenarios**:

1. **Given** a new server form, **When** the user pastes a valid OpenSSH/PEM key and saves,
   **Then** `~/.lazyssh/keys/<alias>` is written with mode 0600 (directory 0700), the key path is
   listed as an `IdentityFile` of the host (prepended if not already present), and `private_key` is stored in `~/.lazyssh/metadata.json`.
2. **Given** a key copied on Windows with `\r\n` line endings, **When** the user saves, **Then**
   the key is accepted and stored with `\n` line endings only, ending with exactly one `\n`.
3. **Given** a passphrase-protected key, **When** the user saves, **Then** it is accepted (the
   passphrase is not needed to store it; ssh prompts for it on connect).
4. **Given** text that is not a private key, **When** the user saves, **Then** the save is
   rejected with `private key is not a valid OpenSSH/PEM private key` and nothing is written.

---

### User Story 2 - Import a key from an existing file (Priority: P2)

Instead of pasting, the user types or autocompletes a path in `Import key from file:` (for example
`~/.ssh/id_ed25519`). On save, if the `PrivateKey:` box is empty, lazyssh reads that file and uses its
content as the embedded key.

**Why this priority**: Convenience on top of P1; most users already have a key file.

**Independent Test**: Leave `PrivateKey:` empty, enter `~/.ssh/<file>` in the import field, save;
the server now has an embedded key equal to the file content.

**Acceptance Scenarios**:

1. **Given** an empty `PrivateKey:` box and an import path, **When** the user saves, **Then** the
   file (with leading `~` expanded to the home directory) is read and becomes the embedded key.
2. **Given** both pasted text and an import path, **When** the user saves, **Then** the pasted text
   wins and the file is not read.
3. **Given** an import path that cannot be read, **When** the user saves, **Then** a modal shows
   `Import key failed: <error>` and the form stays open with nothing saved.

---

### User Story 3 - Keep key files in step with the server list (Priority: P2)

Renaming, clearing the key, or deleting a server keeps the key file and `IdentityFile` in step. When
lazyssh lists servers and a stored key has no file (or a different one) on disk, it rewrites the
file. This is what makes keys appear after a sync pull on another machine.

**Why this priority**: Without it, stale key files accumulate and renamed hosts point at missing
files.

**Independent Test**: Exercise rename, clear, delete and "file deleted behind lazyssh's back" on a
temporary config, checking files and `IdentityFile` after each.

**Acceptance Scenarios**: see the Behaviour table in Requirements; each row is one scenario.

---

### User Story 4 - See that a server uses an embedded key without exposing it (Priority: P3)

The server details pane shows the `Key:` line with an `(embedded)` suffix when the server has an
embedded key. The key text itself is never shown there, in the status bar, or in logs.

**Why this priority**: Visibility only; does not change behaviour.

**Independent Test**: Select a server with an embedded key; the details pane shows `(embedded)` and
no key text.

**Acceptance Scenarios**:

1. **Given** a server with an embedded key, **When** it is selected, **Then** the `Key:` line ends
   with ` (embedded)` and no line contains key text.

---

### Edge Cases

- Alias `.` or `..`: rejected by validation (would make the key path a directory).
- Key with trailing blank lines or trailing spaces: trailing whitespace trimmed, one `\n` appended.
- Key text consisting only of whitespace: treated as no key.
- User already listed the key path in `Keys:` (IdentityFiles): it is not duplicated.
- Key cleared while alias also renamed: old key file removed, old path removed from
  `IdentityFile`, nothing written for the new alias.
- Key file write fails during listing: logged as a warning (alias and error only, no key text); the list
  still loads.
- Removing a key file that does not exist: not an error.
- Saving with an empty key on a server that never had one: the key-file removal and
  `IdentityFile` cleanup still run and are no-ops.

## Requirements *(mandatory)*

### Data

- **FR-001**: Each server MUST be able to carry a private key as text (PEM, may be
  passphrase-protected).
- **FR-002**: The key text MUST be persisted in `~/.lazyssh/metadata.json` under the per-server
  field `private_key`, omitted when empty.
- **FR-003**: The key file path for a server MUST be `<dir of metadata.json>/keys/<alias>`, i.e.
  `~/.lazyssh/keys/<alias>` by default.

### Behaviour

- **FR-004**: The system MUST apply this table:

| Event | Effect |
|---|---|
| Add/Update server with `PrivateKey != ""` | Write key file (0600, dir 0700). Ensure `IdentityFiles` contains the key path (prepend if missing). Persist `private_key` in metadata. |
| Update server with `PrivateKey == ""` and metadata previously had one | Delete key file. Remove key path from `IdentityFiles`. Clear `private_key`. |
| Alias rename with embedded key | Remove old key file, write new one, replace path in `IdentityFiles`. |
| Delete server | Delete key file if present. |
| `ListServers` | For every server whose metadata has `private_key` and whose key file is missing or differs, rewrite the file. This is what makes keys appear after a sync pull on another machine. |

### Validation

- **FR-005**: Alias `.` and `..` MUST be rejected (in addition to the existing
  `^[A-Za-z0-9_.-]+$` rule).
- **FR-006**: A non-empty key MUST parse as an OpenSSH/PEM private key. A key that only fails
  because it needs a passphrase MUST be accepted. Any other failure MUST be reported as
  `private key is not a valid OpenSSH/PEM private key`.
- **FR-007**: Stored key text MUST be normalised: `\r\n` converted to `\n`, trailing whitespace
  trimmed, then exactly one `\n` appended; whitespace-only input becomes empty (no key).

### UI

- **FR-008**: The Authentication tab MUST start with a section `▶ Embedded Private Key (synced)`
  containing:
  - `PrivateKey:` multi-line text box, width 60, height 8, prefilled with the stored key when
    editing.
  - `Import key from file:` single-line field with the existing SSH-key path autocomplete. On Save,
    if `PrivateKey` is empty and this path is non-empty, the file is read (`~` expanded) and its
    content becomes `PrivateKey`, then normalised and validated like pasted text. The path itself
    is not stored and the field is empty whenever the form opens.
- **FR-009**: The server details pane `Key:` line MUST show the suffix `(embedded)` when the server
  has a key. The key text MUST never be rendered there.
- **FR-010**: Field help for `PrivateKey` MUST explain that the key is stored in
  `~/.lazyssh/metadata.json`, written to `~/.lazyssh/keys/<alias>`, and included in sync.

### Security

- **FR-011**: Private key text MUST NOT appear in logs, the status bar, error messages, or the
  details pane. Log lines identify a server by alias only, never by dumping the server record.
- **FR-012**: Files written under `~/.lazyssh` MUST be 0600 and directories 0700.

### Key Entities

- **Server**: an SSH host entry; gains an optional private key (text).
- **Server metadata**: lazyssh-only per-alias data in `metadata.json`; gains `private_key`.
- **Key file**: on-disk copy of the key at `~/.lazyssh/keys/<alias>`, derived from metadata, never
  the source of truth.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After saving a server with a pasted key, `ssh <alias>` from a plain terminal uses that
  key with no further user steps (100% of saves with a valid key).
- **SC-002**: 100% of rows in the Behaviour table are covered by an automated test that passes.
- **SC-003**: Keys pasted with Windows line endings are accepted in 100% of cases and stored with
  Unix line endings.
- **SC-004**: Zero occurrences of key text in log output, status messages, or the details pane
  across the test suite and manual check.
- **SC-005**: Deleting a key file behind lazyssh's back is repaired on the next server list load
  (one refresh).

## Assumptions

- Keys are materialised to disk because OpenSSH needs a file path; this keeps `ssh <alias>` and
  SFTP working unchanged (source spec Assumption 5).
- The metadata file is the source of truth; the key file is a cache rebuilt by listing.
- The existing alias charset `^[A-Za-z0-9_.-]+$` makes the alias filename-safe; only `.` and `..`
  need extra rejection.
- Passphrases are never stored; ssh prompts for them at connect time.
- The plain `PrivateKey:` box shows the key text in the edit form only (needed to edit it); this is
  not a "display" in the sense of FR-011, which covers logs, status bar and details pane.
- Sync (Phase 2) and SFTP (Phase 3) are out of scope; this feature only stores the key where sync
  will later pick it up.
- The `~/.lazyssh` directory itself is created 0700 for new installs (metadata manager and logger
  both used 0750; aligned with FR-012). Existing directories keep their mode, and the log file is
  still created 0644 by zap; key files stay protected by the 0700 `keys/` directory and 0600 files.
- Pinning `golang.org/x/crypto v0.58.0` (source plan) raises the module `go` directive from 1.24.6
  to 1.26.0, because that release requires Go 1.26. CI reads the version from `go.mod`.
