# Data Model: SFTP File Manager

No persisted data. In-memory types only.

## SFTP session (`ports.SFTPSession`)

| Field (impl) | Meaning |
|---|---|
| client `*sftp.Client` | returned by `Client()` |
| cmd `*exec.Cmd` | the `ssh -s -- <alias> sftp` subprocess |
| stdin `io.WriteCloser` | pipe into ssh |
| stderr tail | last 4 KiB of ssh stderr (for errors and logs) |

Lifecycle: `Connect` → open (handshake done) → `Close` (client closed, process killed and reaped).
A failed `Connect` leaves no process.

## fileEntry (`ui`)

| Field | Type | Rule |
|---|---|---|
| Name | string | base name; `..` for the parent row |
| IsDir | bool | true for directories and symlinks to directories |
| Size | int64 | bytes; directories show no size |
| ModTime | time.Time | shown as `01-02` |

Ordering (`sortEntries`): directories first, then files; each group by `strings.ToLower(Name)`.

## filePane (`ui`)

`*tview.Table` + `title`, `cwd`, `list func(string) ([]fileEntry, error)`, `join func(...string)
string`. Rows: 0 = header (`Name`, `Size`, `Modified`, fixed), 1 = `../`, 2.. = sorted entries.
`Load(p)` replaces rows only on success.

## TransferProgress (`services`)

| Field | Meaning |
|---|---|
| File | path of the current file relative to the transferred root (base name for a single file) |
| Done / Total | bytes copied / size of the current file |
| FileIndex / FileCount | 1-based index of the current file / number of files in the transfer |
