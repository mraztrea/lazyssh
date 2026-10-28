# Research: Settings File and Encrypted Sync

No NEEDS CLARIFICATION items remained after the spec; these notes record the decisions the source
plan left implicit.

## R1: Key derivation from the standard library

- **Decision**: `crypto/pbkdf2.Key(sha256.New, password, salt, iter, 32)` (returns `([]byte,
  error)`, Go 1.24+). AES-256-GCM via `crypto/aes` + `crypto/cipher.NewGCM` (12-byte nonce).
- **Rationale**: Spec forbids extra crypto libraries; the toolchain (go 1.26) has `crypto/pbkdf2`.
- **Alternatives**: `golang.org/x/crypto/pbkdf2` (already a dependency, but deprecated in favour of
  stdlib); scrypt/argon2 (not in the spec's envelope `kdf` field).

## R2: Zero times in settings.json

- **Decision**: `last_push_at` / `last_pull_at` use `json:",omitzero"`; zero times are omitted
  instead of written as `""` or `0001-01-01T00:00:00Z`.
- **Rationale**: `omitempty` never omits a `time.Time` struct, and `time.Time` cannot unmarshal
  `""`. lazyssh is the only writer of this file, so the spec's illustrative `"last_pull_at": ""`
  never appears on disk.
- **Alternatives**: custom time type accepting `""` (more code for a value lazyssh never writes).

## R3: Atomic writes for metadata on Import and for the file backend

- **Decision**: write to a temp file in the same directory (0600) and `os.Rename` over the target.
  Config uses the repository's existing temp + rename path (extracted into `saveRawConfig(content)`, which also takes the backups; the source plan called it `writeRawToFile`).
- **Rationale**: Spec requires atomic replacement; rename within a directory is atomic on POSIX.
- **Ceiling**: config and metadata are two renames, not one transaction; a crash between them
  leaves the new config with the old metadata (recoverable by pulling again). Marked `ponytail:`.

## R4: Where the backend factory lives

- **Decision**: a `backendFor(domain.SyncSettings) (ports.SyncBackend, error)` function in
  `cmd/main.go`; `services.ErrSyncNotConfigured` is returned for empty/unknown providers.
  `NewSyncService` receives the function.
- **Rationale**: The source plan says the factory "lives in `cmd/main.go` wiring"; constitution I
  forbids `services` importing adapters, so it cannot be a `services` function.
- **Alternatives**: a registry map or a factory interface (one implementation; rejected by V).

## R5: Adapter package named `sync`

- **Decision**: keep `internal/adapters/sync` (package `sync`) as named in the source plan; import
  it in `cmd/main.go` as `syncadapter`.
- **Rationale**: Matches plan file paths; the alias avoids shadowing stdlib `sync`.

## R6: Gist API details

- **Decision**: JSON body `{"description":"lazyssh sync","public":false,"files":{"lazyssh-sync.enc":
  {"content":"<envelope>"}}}` for POST and PATCH (PATCH sends `files` only). Response fields used:
  `id`, `files[name].content`, `.truncated`, `.raw_url`, and `message` on errors. The `raw_url`
  request is sent without the `Authorization` header.
- **Rationale**: GitHub REST v3 gist API; secret gist raw URLs are capability URLs that need no
  token, so not sending it avoids leaking the token to a second host.
- **Alternatives**: base64 the envelope (not needed: envelope is ASCII JSON).

## R7: Service tests use fakes

- **Decision**: `sync_service_test.go` uses in-memory fakes for settings, snapshot and backend.
- **Rationale**: Source plan suggested the real settings repo; importing an adapter from the
  `services` package contradicts constitution I. The real repo is already covered by Task 4 tests.
