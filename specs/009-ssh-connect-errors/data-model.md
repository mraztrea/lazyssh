# Data model: Báo lỗi kết nối SSH/SFTP

Không có dữ liệu lưu bền. Hai kiểu giá trị sống từ lúc ssh thoát đến lúc hộp thoại đóng.

## `domain.SSHError` (mới, `internal/core/domain/ssh_error.go`)

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `Reason` | `string` | Nguyên nhân một dòng, tiếng Anh (`Connection refused`, …) |
| `Hint` | `string` | Gợi ý xử lý một câu; có thể rỗng |
| `Output` | `string` | Tối đa 8 dòng cuối không rỗng của stderr ssh, đã bỏ `\r`; rỗng thì là văn bản dự phòng (`ssh exited with status 255`, hoặc lỗi gốc) |
| `HostKey` | `*HostKeyChange` | Chỉ khác `nil` khi host key đổi VÀ đọc được cả tên host lẫn file |

`Error()` trả `Reason`. Không bao giờ chứa mật khẩu/khoá: chỉ chép đầu ra của ssh và thông báo lỗi
đọc metadata.

## `domain.HostKeyChange` (mới)

| Trường | Kiểu | Nguồn trong thông báo ssh |
|---|---|---|
| `Host` | `string` | `Host key for <Host> has changed` (ví dụ `103.142.24.161`, `[1.2.3.4]:2222`) |
| `File` | `string` | `Offending <TYPE> key in <File>:<Line>` |
| `Line` | `int` | như trên |
| `KeyType` | `string` | `The fingerprint for the <KeyType> key sent by the remote host is` |
| `Fingerprint` | `string` | dòng kế tiếp, bỏ dấu `.` cuối |

## Hàm (`internal/core/services/ssh_error.go`)

| Hàm | Vào | Ra |
|---|---|---|
| `classifySSHError(stderr, fallback string) *domain.SSHError` | stderr thô của ssh, văn bản dự phòng | lỗi đã phân loại theo bảng R3 trong [research.md](./research.md) |
| `storedPasswordError(err error) *domain.SSHError` | lỗi đọc mật khẩu | `Reason: "Stored password unavailable"` |

## Port `ServerService` (sửa)

- `SSH(server)`, `SSHWithArgs(server, args)`: trả `nil` khi phiên kết thúc bình thường hoặc bị ngắt
  bằng tín hiệu; `*domain.SSHError` khi ssh thoát 255 hoặc không đọc được mật khẩu đã lưu; lỗi gốc
  khi không chạy được `ssh`. `RecordSSH` chỉ khi mã 0 (như cũ).
- `RemoveKnownHost(host, file string) error` (mới): `ssh-keygen -R host -f file`; lỗi kèm đầu ra
  của `ssh-keygen`.

## `SFTPService.Connect` (sửa)

Bắt tay thất bại → `*domain.SSHError` từ `classifySSHError(stderr, lỗi gốc)`; lỗi đọc mật khẩu →
`storedPasswordError`.

## Luồng hộp thoại

```text
ssh/sftp lỗi ──► SSHError.HostKey != nil ? ──có──► [Cancel*] [Remove old key & reconnect]
                     │                                   │Cancel/Esc → danh sách
                     │                                   │Remove → RemoveKnownHost ─ok─► kết nối lại (ssh hỏi fingerprint)
                     │                                                          └─lỗi─► hộp thoại lỗi thường
                     └─không─► [Retry] [Edit server] [Close*]
                                 Retry → kết nối lại cùng cách; Edit → form sửa server; Close/Esc → danh sách
(* = nút được chọn sẵn)
```
