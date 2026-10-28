# Quickstart: Validate SFTP File Manager

## Automated

```sh
make test                      # all packages, -race
go test ./internal/core/services/ -run 'TestConnect|Upload|Download|RemoveRemote' -v
go test ./internal/adapters/ui/ -run 'SortEntries|FormatSize|FilePane' -v
make build                     # fmt, vet, golangci-lint, binary
```

`TestConnect_ProcessExitReturnsError` pins "ssh exiting immediately returns an error, not a hang".

## Manual (tmux, throwaway HOME)

Needs an alias that reaches an sshd with SFTP and authenticates without a prompt (e.g. a local
`sshd` on a high port with a generated host key and authorized key).

```sh
export H=$(mktemp -d) && mkdir -p $H/.ssh
printf 'Host t\n  HostName 127.0.0.1\n  Port 2222\n  User %s\n  IdentityFile %s\n' "$USER" /path/to/key > $H/.ssh/config
HOME=$H ./bin/lazyssh          # inside tmux
```

1. Select `t`, press `m`: left `Local: $H`, right `Remote: <remote home> (t)`, `../` first,
   directories before files.
2. `j`/`k`, `Enter` into a directory, `h` back, `Tab` switches pane, `g` + path jumps.
3. Local pane: select a file, `u` → progress line, then green `Uploaded <name>`; it appears in the
   remote pane. `u` again → `Overwrite <name>?`.
4. Remote pane: `d` on that file (after deleting/renaming the local copy) → `Downloaded <name>`.
5. `n` → `New directory:` → `x`; `r` on `x` → `y`; `D` on `y` → `Delete y?` → Delete.
6. `q` → main screen; `pgrep -af '[s]sh -s --'` prints nothing (the bracket keeps pgrep from matching its own
   shell).
7. Alias pointing at a closed port → `m` → red `SFTP failed: … Connection refused`.
