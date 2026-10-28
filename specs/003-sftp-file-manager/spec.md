# Feature Specification: SFTP File Manager

**Feature Branch**: `003-sftp-file-manager`

**Created**: 2026-10-28

**Status**: Implemented

**Input**: User description: "SFTP file manager: open a dual-pane local/remote file browser for the selected server over the system ssh client (ssh -s sftp alias) with upload, download, mkdir, rename, delete, go-to-path and progress display."

**Source document**: `docs/2026-10-28-sync-keys-sftp-spec.md`, section "Phase 3: SFTP file manager" (plus "Assumptions" #4, "Global constraints", "Out of scope").

## Clarifications

### Session 2026-10-28

Answered non-interactively from the source spec and `docs/2026-10-28-sync-keys-sftp-plan.md`.

- Q: What does the user see when the connection fails? → A: The main screen with a red status
  `SFTP failed: <err>`, where `<err>` ends with the last stderr line ssh printed (source spec:
  "Failure message includes the last stderr line"; the file manager screen never opened).
- Q: What does the progress line show for a directory transfer? → A: The current file name, then
  `n/total files`, then percent and speed, e.g. `Uploading c.txt  3/10 files  45%  2.1 MB/s`; a
  single file omits the counter (source spec progress format plus "progress shows `n/total files`").
- Q: What is shown when a transfer finishes? → A: Green `Uploaded <name>` / `Downloaded <name>` on
  the file manager's status line, both panes refreshed; errors in red as `Upload failed: <err>` /
  `Download failed: <err>` (source spec "Transfer error → red status, panes refreshed").
- Q: When the destination of a directory transfer already exists, how often is the user asked? →
  A: Once, `Overwrite <name>?` for the top-level name; on confirm, files inside are overwritten and
  other existing files kept (simplest reading of "Existing destination file → confirm modal").
- Q: Can mkdir/rename/delete/navigation be used while a transfer runs? → A: Yes; only transfers are
  limited to one at a time (source spec "One transfer at a time"; "Multiple parallel transfers" is
  out of scope).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse a server's files next to my local files (Priority: P1)

A user selects a server in the list and presses `m`. lazyssh hands the terminal to OpenSSH (so a
passphrase, password or host-key prompt can appear), opens an SFTP session through the user's own
SSH configuration for that alias, then shows two panes side by side: local files on the left
(starting in the home directory) and remote files on the right (starting in the remote login
directory). The user moves with `j`/`k`/arrows, enters directories with `Enter`, goes up with
`Backspace`/`h` or the `..` row, switches panes with `Tab`, jumps with `g` to a typed path, and
presses `q`/`Esc` to close the session and return to the server list.

**Why this priority**: Browsing is the base every other action needs; on its own it already
answers "what is on that box?" without opening a shell.

**Independent Test**: Against a reachable host, press `m`; both panes list their start directories
with directories first; navigate in both; press `q`; the main screen is back and no ssh subprocess
started by the file manager remains.

**Acceptance Scenarios**:

1. **Given** a selected server whose alias is reachable, **When** the user presses `m`, **Then** the
   file manager opens with the local pane at the user's home directory and the remote pane at the
   remote working directory, each titled with its path (remote title also names the alias).
2. **Given** an open file manager, **When** the user presses `Enter` on a directory or `..`,
   **Then** that pane lists the new directory; `Backspace`/`h` lists the parent.
3. **Given** an open file manager, **When** the user presses `g` and enters a path, **Then** the
   active pane lists that path, or shows a red error and stays where it was if the path cannot be
   listed.
4. **Given** an alias that cannot connect (auth failure, unknown host, ssh exits), **When** the user
   presses `m`, **Then** the app returns to the main screen with a red status message
   `SFTP failed: <err>` that ends with the last line ssh printed on stderr, and no `ssh` process is
   left behind.
5. **Given** an open file manager, **When** the user presses `q` or `Esc`, **Then** the session is
   closed, the ssh process is gone, and the main screen is shown.

---

### User Story 2 - Upload and download files and directories (Priority: P1)

In the file manager the user selects a local file or directory and presses `u` to copy it into the
remote pane's current directory, or selects a remote entry and presses `d` to copy it into the local
pane's current directory. A progress line shows the transfer; directories are copied file by file
with an `n/total files` counter. If the destination already exists, the user is asked to confirm
the overwrite first.

**Why this priority**: Moving files is the reason to open a file manager on a server.

**Independent Test**: Upload a 3 MiB local file and a nested directory to a test SFTP server and
download them back; contents and file modes match and progress reaches 100%.

**Acceptance Scenarios**:

1. **Given** a local file selected, **When** the user presses `u`, **Then** it is copied to the
   remote current directory under the same name, the progress line reads like
   `Uploading file.txt  45%  2.1 MB/s` while running, and both panes refresh when it ends.
2. **Given** a local directory selected, **When** the user presses `u`, **Then** the directory tree
   is recreated remotely and every file is copied, the progress line also showing `n/total files`.
3. **Given** a remote file or directory selected, **When** the user presses `d`, **Then** it is
   copied into the local current directory the same way (`Downloading …`).
4. **Given** the destination name already exists, **When** the user starts the transfer, **Then** a
   modal `Overwrite <name>?` appears; cancelling copies nothing, confirming overwrites.
5. **Given** a transfer is running, **When** the user presses `u` or `d` again, **Then** no second
   transfer starts and the status line says a transfer is already running.
6. **Given** a transfer fails part-way, **Then** the status line shows the error in red, both panes
   are refreshed, and the partial file is left in place.

---

### User Story 3 - Manage files on either side (Priority: P2)

In the active pane the user presses `n` to create a directory (prompted for a name), `r` to rename
the selected entry (prompted, pre-filled with the current name), `D` to delete the selected entry
after a confirm modal that names it (directories are deleted recursively), and `R` to refresh both
panes.

**Why this priority**: Useful housekeeping, but browsing and transfers deliver value without it.

**Independent Test**: In each pane create a directory, rename it, delete it with confirm; the
listing reflects each step.

**Acceptance Scenarios**:

1. **Given** the active pane, **When** the user presses `n` and enters `new`, **Then** directory
   `new` exists in that pane's current directory and is shown.
2. **Given** an entry selected, **When** the user presses `r` and enters a new name, **Then** the
   entry is renamed within the same directory.
3. **Given** an entry selected, **When** the user presses `D`, **Then** a modal asks
   `Delete <name>?`; confirming removes it (recursively for directories), cancelling does nothing.
4. **Given** any operation fails (permission denied, name exists), **Then** the status line shows
   the error in red and the listing is unchanged.

### Edge Cases

- The `..` row is never uploaded, downloaded, renamed or deleted; those keys do nothing on it.
- At the filesystem root `..` is still shown; entering it stays at the root.
- `Enter` on a file does nothing (no viewer/editor).
- Symbolic links to directories can be entered (listing follows the link target); deleting a
  symbolic link removes only the link, never the target's contents.
- Special files (sockets, devices, links to non-regular files) inside a transferred directory are
  skipped; empty directories are recreated.
- A local home directory that cannot be determined falls back to `/`.
- A prompt submitted empty (or with `/` in a mkdir/rename name) does nothing.
- Pressing `q` while a transfer runs closes the session; the transfer stops with an error and the
  partial file is left in place.

## Requirements *(mandatory)*

### Connection

- **FR-001**: The session MUST be opened by running the system `ssh` binary requesting the `sftp`
  subsystem (source wording: `ssh -s sftp <alias>`; actual argv `ssh -s -- <alias> sftp`, see
  Assumptions) with the SFTP protocol spoken over its stdin/stdout, so every option in the
  user's SSH config (ProxyJump, agent, ControlMaster, passphrase prompts) applies. No in-process SSH
  client.
- **FR-002**: Connecting MUST happen while the TUI is suspended so OpenSSH can prompt on the
  terminal.
- **FR-003**: If ssh exits before the SFTP handshake completes (auth failure, unknown host), the
  connect MUST return an error promptly (within 5 s of the exit) instead of hanging, and the error
  MUST include the last line ssh wrote to stderr. ssh stderr is otherwise sent to the log.
- **FR-004**: Closing the file manager MUST close the SFTP client and kill the ssh subprocess; no
  ssh subprocess started by the file manager may remain after `q`.
- **FR-005**: The transport is a deliberate shortcut, marked in code with the comment
  `ponytail:` subprocess transport. Upgrade to `golang.org/x/crypto/ssh` only if an in-process
  client is ever required.

### Layout

- **FR-006**: Global key `m` on a selected server opens the file manager. Layout:

```
┌ Local: /home/me/projects ───────┐┌ Remote: /var/www (alias) ────────┐
│ ../                             ││ ../                              │
│ dir/                            ││ app/                             │
│ file.txt           1.2K  10-28  ││ index.html        4.1K  10-27    │
└─────────────────────────────────┘└──────────────────────────────────┘
 status / transfer progress line
```

- **FR-007**: Each pane is a table (name, size, mtime) with a `..` row first. Directories first,
  then files, both case-insensitive by name. Directory names end in `/`. Sizes are shown as `0`,
  `512`, `1.2K`, `3.3M`, `1.1G` (1024-based, one decimal from K upward).
- **FR-008**: Start directories: local = `os.UserHomeDir()`, remote = `client.Getwd()`.

### Keys

- **FR-009**: Keys inside the file manager:

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

- **FR-010**: `u` acts on the local pane's selection and `d` on the remote pane's selection
  regardless of which pane is active.

### Transfers

- **FR-011**: One transfer at a time, run in a goroutine. Progress line:
  `Uploading file.txt  45%  2.1 MB/s`. Directories are walked and transferred file by file;
  progress shows `n/total files`, placed after the file name
  (`Uploading c.txt  3/10 files  45%  2.1 MB/s`). On success the status line shows green
  `Uploaded <name>` / `Downloaded <name>`.
- **FR-012**: Existing destination file → confirm modal `Overwrite <name>?`. For a directory the
  question is asked once for the top-level name; confirming overwrites files inside it and keeps
  other existing files.
- **FR-013**: Copy uses 1 MiB buffer and `io.Copy`. File mode is preserved (`chmod` after write,
  best effort).
- **FR-014**: Transfer error → red status, panes refreshed; partial file left in place (user can
  retry).
- **FR-015**: The destination name is the source's base name inside the destination directory.

### File operations

- **FR-016**: `n` creates a directory, `r` renames within the current directory, `D` deletes after
  a confirm modal naming the entry; directories are deleted recursively. All work on the active
  pane's side (local filesystem or remote server) and stay available while a transfer runs.

### Safety

- **FR-017**: All UI changes from background work go through the UI thread (`QueueUpdateDraw`).
- **FR-018**: File contents are never logged. Paths, sizes and errors may be logged.

### Key Entities

- **SFTP session**: an open SFTP client plus the ssh subprocess behind it; closed together.
- **File entry**: name, is-directory flag, size, modification time; one row in a pane.
- **Transfer progress**: current file, bytes done/total of that file, file index and file count.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: From the server list, a user reaches a listing of the remote home directory with one
  key press (plus any prompt OpenSSH itself shows).
- **SC-002**: A file uploaded and downloaded back is byte-identical and keeps its permission bits.
- **SC-003**: A failed connection returns the user to the server list with the reason within 5 s of
  ssh exiting.
- **SC-004**: After closing the file manager, zero ssh subprocesses started by it remain.
- **SC-005**: The progress line updates while a multi-megabyte file transfers and reaches 100% when
  it completes.

## Assumptions

- SFTP goes through the system `ssh` binary (source Assumption 4); hosts that need options lazyssh
  does not know about still work because the user's SSH config is used as-is.
- The status/progress line belongs to the file manager screen; when idle it shows the key hints.
  Connect failures are shown in the main screen's status bar because the file manager never opened.
- Directory modes are not preserved (created with default permissions); file permission bits are.
- Local directories created by `n` or by a download use mode 0750 (gosec G301 limit; user files,
  not `~/.lazyssh`).
- Speed (MB/s) is measured per file since that file started (1 MB = 1,000,000 bytes).
- Progress updates are throttled to about 10 per second so a fast link does not flood the UI.
- Implementation deviations from the source plan (all recorded in research.md / tasks.md):
  - The source's `ssh -s sftp <alias>` does not work: OpenSSH's `-s` takes no argument, so `sftp`
    is parsed as the host (found during manual verification: "Could not resolve hostname sftp").
    The command is `ssh -s -- <alias> sftp`; `--` stops an alias beginning with `-` from being read
    as an option (aliases can arrive through a pulled sync bundle). Pinned by
    `TestNewSFTPService_RequestsSubsystemAfterAlias`, committed as a separate `fix:` commit.
  - The parent row's cell text is `../` (spec layout sketch); the source plan test's `row1 == ".."`
    is checked as `Selected().Name == ".."`.
  - `newFilePane(title, list)` keeps the plan signature; the `join` func is a pane field defaulting
    to `filepath.Join` and set to `path.Join` for the remote pane; `ParentDir()` is
    `join(cwd, "..")`.
  - The `ponytail:` comment uses this spec's wording (FR-005), not the plan's paraphrase.
  - "1 MiB buffer and `io.Copy`" is implemented with `io.CopyBuffer` (io.Copy with an explicit
    buffer) so the buffer size and progress counting actually apply (research R4).
  - The UI calls `services.UploadPath/DownloadPath/RemoveRemote` directly as the plan defines them;
    recorded in plan Complexity Tracking against constitution Principle I.
- Out of scope: multiple parallel transfers, resume, drag and drop, in-process SSH, file viewer or
  editor, multi-select.
