# Quickstart: Validate Settings File and Encrypted Sync

## Automated

```sh
make test                      # all packages, -race
go test ./internal/adapters/data/settings_file/ ./internal/adapters/sync/ -v
go test ./internal/adapters/data/ssh_config_file/ -run 'Export|Import' -v
go test ./internal/core/services/ -run 'Seal|Open|TestSync' -v
make build                     # fmt, vet, golangci-lint, binary
```

`TestSync_PullWrongPassword_DoesNotImport` pins "wrong password never touches local files".

## Manual (throwaway HOME, provider `file`)

```sh
export H=$(mktemp -d) && mkdir -p $H/.ssh
printf 'Host demo\n  HostName 10.0.0.1\n  User alice\n' > $H/.ssh/config
HOME=$H ./bin/lazyssh          # run inside tmux to drive it
```

1. `y` → Provider `file`, File path `/tmp/lazyssh-sync.enc`, Password `x` → `Push`.
   Expect status `Pushed (file)`; `grep -c Host /tmp/lazyssh-sync.enc` → `0`; file mode 0600;
   `$H/.lazyssh/settings.json` has `last_push_at` and no password.
2. `e` on `demo`, change User to `bob`, save. `cat $H/.ssh/config` shows `bob`.
3. `y` → Password `x` → `Pull` → confirm. Expect status `Pulled (file)`; the list/details show
   `alice` again; `ls $H/.ssh/` shows a new `config-<ms>-lazyssh.backup` containing `bob`.
4. `y` → Password `wrong` → `Pull` → confirm. Expect red `Pull failed: wrong password or corrupted
   sync data`; config unchanged, no new backup.
