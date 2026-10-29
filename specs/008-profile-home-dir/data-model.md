# Data model: Thư mục làm việc mặc định (Home Dir)

## Thực thể

### `domain.Server` (sửa)

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `HomeDir` | `string` | Thư mục làm việc trên máy remote; rỗng = không áp dụng. Tuyệt đối (`/var/www`), `~`, `~/x` hoặc tương đối (tính từ home remote). Không chứa `\n`/`\r` (`validateServer`). Đã trim. |

### `ServerMetadata` trong `~/.lazyssh/metadata.json` (sửa)

```json
{
  "web-prod": {
    "tags": ["prod"],
    "home_dir": "/var/www/app"
  }
}
```

- Khoá `home_dir`, `omitempty`: rỗng thì không ghi.
- Đi theo alias: `updateServer` đổi alias thì chuyển cả entry (đã có).
- Nằm trong `SyncBundle.Metadata` (toàn bộ file) nên được push/pull không cần sửa sync.
- File cũ không có khoá → đọc như rỗng.

## Hàm thuần (`internal/core/services/home_dir.go`)

| Hàm | Vào | Ra |
|---|---|---|
| `HomeDirCommand(s domain.Server) string` | `HomeDir`, `RemoteCommand` | `""` nếu HomeDir rỗng; ngược lại `cd <expr>; exec "$SHELL" -l` hoặc `cd <expr>; <RemoteCommand>` (RemoteCommand khác rỗng và khác `none`) |
| `ResolveRemoteDir(dir, home string) string` | Home Dir, thư mục làm việc SFTP | `home` nếu rỗng/`~`; `path.Join(home, rest)` cho `~/rest` và đường dẫn tương đối; `path.Clean(dir)` cho tuyệt đối |

`<expr>`: `"$HOME"` cho `~`; `"$HOME"/'rest'` cho `~/rest`; `'dir'` (single-quote, `'` → `'\''`) cho
còn lại.

## Hợp đồng dòng lệnh ssh (`sshArgs`)

| Trường hợp | argv |
|---|---|
| Không Home Dir | `ssh <pre…> <auth…> <post…> <alias>` (y hệt hiện tại) |
| Có Home Dir | `ssh -o RemoteCommand=none -t <pre…> <auth…> <post…> <alias> '<HomeDirCommand>'` (lệnh là một phần tử argv, không qua shell cục bộ) |
| Port forward nền (`StartForward`) | không bao giờ thêm lệnh remote |

## Luồng trạng thái

Không có; giá trị tĩnh, đọc lúc kết nối.
