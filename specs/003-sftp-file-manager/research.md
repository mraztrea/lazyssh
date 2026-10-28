# Research: SFTP File Manager

No NEEDS CLARIFICATION remained in Technical Context; these record the design choices.

## R1 Transport: system ssh subprocess

- Decision: `exec.Command("ssh", "-s", "--", alias, "sftp")` (OpenSSH `-s` takes no argument: host
  first, then subsystem; the source's `ssh -s sftp <alias>` connects to a host named `sftp`) with stdin/stdout pipes into
  `sftp.NewClientPipe(stdout, stdin)`; connect inside `app.Suspend`.
- Rationale: source Assumption 4; the user's SSH config (ProxyJump, agent, ControlMaster,
  passphrase/password/host-key prompts on `/dev/tty`) works unchanged. The subprocess is not
  detached (unlike port forwards) because it must share the terminal for prompts.
- Alternatives: `golang.org/x/crypto/ssh` in-process client — rejected by the spec (out of scope);
  marked with the `ponytail:` comment as the upgrade path.

## R2 Error reporting when ssh dies

- Decision: `cmd.Stderr` is a writer keeping the last 4 KiB. When ssh exits before the SFTP
  version handshake, `NewClientPipe` sees EOF on stdout and returns an error at once; `Connect`
  kills/waits the process (so the stderr copy goroutine has finished), logs the tail and returns
  the error with the last non-empty stderr line appended.
- Rationale: Review Focus 4 ("must return an error, not hang"); spec FR-003.
- Alternatives: a separate goroutine racing `cmd.Wait()` against the handshake — unnecessary,
  because stdout EOF already unblocks the handshake.

## R3 Close semantics

- Decision: `client.Close()`, `stdin.Close()`, `Process.Kill()`, `Wait()`; ignore the
  `signal: killed` wait error; `sync.Once` so a second `Close` is harmless.
- Rationale: FR-004 (no leaked processes after `q`).

## R4 Copy loop

- Decision: `io.CopyBuffer(counter{dst}, struct{ io.Reader }{src}, make([]byte, 1<<20))`.
- Rationale: wrapping both sides hides `ReaderFrom`/`WriterTo` (`*os.File` and `*sftp.File`
  implement them), so the 1 MiB buffer from the spec is actually used and every write passes the
  progress counter. `sftp.File.Read` with a large buffer still pipelines requests internally.
  `io.CopyBuffer` is `io.Copy` with an explicit buffer.

## R5 Directories and symlinks

- Decision: plan-then-copy. Walk first (`filepath.WalkDir` for upload, `c.Walk` for download),
  collect directories and regular files (symlink entries are `Stat`ed; links to regular files are
  copied as files, others skipped), then copy so `FileCount` is exact. The download root is
  resolved with `c.RealPath` because `c.Walk` uses `Lstat` and would not descend a symlinked root;
  `filepath.WalkDir` already follows a symlinked root. `RemoveRemote` uses `Lstat`/`ReadDir` (lstat
  semantics) so deleting a link never deletes the target's contents; locally `os.RemoveAll` behaves
  the same.
- Listing: entries that are symlinks are `Stat`ed so links to directories can be entered.

## R6 Pane parent directory

- Decision: one `join func(...string) string` per pane (`filepath.Join` local, `path.Join`
  remote); `ParentDir() = join(cwd, "..")`.
- Rationale: plan says "pass a join func into the pane"; `Join(x, "..")` equals `Dir(x)` for the
  cleaned absolute paths the pane holds, and stays at the root (`/` → `/`), so no second func is
  needed. `newFilePane(title, list)` keeps the plan signature; the default `join` is
  `filepath.Join` and the remote pane overrides the field.

## R7 Overwrite and concurrency

- Decision: before a transfer the UI stats the destination (`c.Stat` / `os.Stat` of
  `dir/base(src)`); exists → `Overwrite <name>?` modal. A single `atomic.Bool` blocks a second
  transfer. Other operations use the same `*sftp.Client`, which is safe for concurrent use.
