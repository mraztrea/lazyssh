# Quickstart: validate Embedded Private Keys

## Automated

```bash
make test                       # go test -race ./...
make build                      # gofumpt, vet, golangci-lint, build
go test ./internal/core/services/ -run 'TestValidateServer|TestNormalize' -v
go test ./internal/adapters/data/ssh_config_file/ -run 'PrivateKey|KeyFile' -v
go test ./internal/adapters/ui/ -run TestResolvePrivateKey -v
```

Expected: all PASS.

## Manual (uses a throwaway HOME so real config is untouched)

```bash
export HOME=$(mktemp -d) && mkdir -p ~/.ssh && touch ~/.ssh/config
ssh-keygen -t ed25519 -N '' -f ~/.ssh/id_test -q
go run ./cmd
```

1. `a` (add), Alias `web`, Host `127.0.0.1`; Authentication tab; `Import key from file:` →
   `~/.ssh/id_test`; Save.
2. Check: `stat -c %a ~/.lazyssh/keys/web` → `600`; `stat -c %a ~/.lazyssh/keys` → `700`;
   `grep IdentityFile ~/.ssh/config` → `.../.lazyssh/keys/web`; `grep private_key
   ~/.lazyssh/metadata.json` → present.
3. Details pane `Key:` line ends with `(embedded)`; no key text visible.
4. `e` (edit): PrivateKey box shows the key. Rename alias to `web2`, Save → `keys/web` gone,
   `keys/web2` present, IdentityFile updated.
5. `rm ~/.lazyssh/keys/web2`, refresh list → file reappears.
6. Edit, clear the PrivateKey box, Save → file and IdentityFile line gone.
7. `grep -r "BEGIN OPENSSH" ~/.lazyssh/*.log 2>/dev/null` → no output (no key text in logs).
