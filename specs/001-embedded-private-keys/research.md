# Research: Embedded Private Keys

## R1. Key parsing library and version

- **Decision**: `golang.org/x/crypto v0.58.0`, `ssh.ParseRawPrivateKey`; accept
  `*ssh.PassphraseMissingError` via `errors.As`.
- **Rationale**: Pinned by the source plan's Global Constraints; parses OpenSSH, PKCS#1, PKCS#8 and
  SEC1 PEM; reports encrypted keys with a distinct error type so they can be accepted without a
  passphrase. Also a transitive dependency of `github.com/pkg/sftp` in Phase 3.
- **Consequence**: v0.58.0's `go.mod` declares `go 1.26.0`, so `go get` raises lazyssh's `go`
  directive from 1.24.6 to 1.26.0. The CI test workflow reads `go-version-file: go.mod` and the local
  toolchain is go1.26.9, so builds keep working. Recorded as a deviation from "Go 1.24".
- **Alternatives considered**: x/crypto v0.47.0 (last with `go 1.24.0`) would keep the directive but
  contradicts the explicitly pinned version and Phase 3's pin; hand-rolled PEM header check would
  not catch malformed bodies.

## R2. Where the key file lives and who owns it

- **Decision**: `filepath.Join(filepath.Dir(metadataPath), "keys", alias)`, owned by the
  `ssh_config_file` repository through an unexported `keyStore`.
- **Rationale**: The repository already owns both `~/.ssh/config` and `metadata.json` and is the only
  place that sees old and new alias together on rename. Deriving the dir from the metadata path makes
  tests use `t.TempDir()` with no extra parameter.
- **Alternatives considered**: a new port + adapter (rejected, one implementation, constitution V);
  writing from the service (rejected, services do no file I/O).

## R3. Normalisation

- **Decision**: `\r\n`→`\n`, `strings.TrimRight(s, " \t\r\n")`-style trailing trim, append `\n`;
  blank → `""`. Applied in the service before validation result is stored.
- **Rationale**: OpenSSH rejects keys without a trailing newline in some versions; Windows clipboard
  yields CRLF (Review Focus 1).

## R4. Logging safety

- **Decision**: Service log calls use `"alias", server.Alias` instead of `"server", server`.
- **Rationale**: zap would serialise the whole struct including `PrivateKey` (constitution IV,
  FR-011). Not listed in the source plan; required by the spec's security constraint.

## R5. UI widget

- **Decision**: tview `TextArea` 60×8 created with `tview.NewTextArea()` and added via
  `form.AddFormItem`, so a focus handler can show field help (equivalent to `form.AddTextArea`, which
  offers no focus hook). Read back raw (no trim) so an unchanged key does not count as an edit.
- **Rationale**: matches existing `addInputFieldWithHelp` pattern.
