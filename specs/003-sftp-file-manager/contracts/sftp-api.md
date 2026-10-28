# Contract: SFTP ports, transfer helpers, file manager keys

## Ports (`internal/core/ports/services.go`)

```go
// SFTPSession is an open SFTP client backed by an ssh subprocess.
type SFTPSession interface {
	Client() *sftp.Client
	Close() error
}

type SFTPService interface {
	Connect(alias string) (SFTPSession, error)
}
```

- `Connect` runs `ssh -s -- <alias> sftp`. Error: `sftp <alias>: <cause>[: <last ssh stderr line>]`.
- `Close` closes the client and kills/reaps the subprocess; safe to call twice.

## Services (`internal/core/services`)

```go
func NewSFTPService(logger *zap.SugaredLogger) ports.SFTPService

type TransferProgress struct {
	File                 string
	Done, Total          int64
	FileIndex, FileCount int
}

func UploadPath(c *sftp.Client, localPath, remoteDir string, progress func(TransferProgress)) error
func DownloadPath(c *sftp.Client, remotePath, localDir string, progress func(TransferProgress)) error
func RemoveRemote(c *sftp.Client, path string) error
```

- Destination = `<destDir>/<base(src)>`; existing files are truncated and overwritten.
- `progress` may be nil; it is called from the transferring goroutine after every buffer write.
- File permission bits copied after write, errors ignored.
- `RemoveRemote` removes files, empty and non-empty directories; symlinks are removed, never
  followed.

## UI

- `NewTUI(logger, serverService, syncService, sftpService, version, commit)`.
- Main screen key `m` → file manager for the selected server; status bar hint `m Files`.
- File manager keys: see spec FR-009.
- Status line texts: `Uploading <file>  [n/total files  ]NN%  X.X MB/s`,
  `Uploaded <name>` / `Downloaded <name>` (green), `Upload failed: <err>` /
  `Download failed: <err>` (red), `A transfer is already running` (red).
- Modals: `Overwrite <name>?` (Cancel / Overwrite), `Delete <name>?` (Cancel / Delete).
- Prompts: `New directory:`, `Rename to:` (pre-filled), `Go to path:` (pre-filled with cwd).
