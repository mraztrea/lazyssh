# Hợp đồng: lazyssh làm helper SSH_ASKPASS và môi trường khởi chạy ssh

## Khởi chạy (lazyssh → ssh), chỉ cho server có `GetPassword(alias)` khác rỗng

Biến môi trường thêm vào (nối sau `os.Environ()`):

| Biến | Giá trị |
|---|---|
| `SSH_ASKPASS` | đường dẫn tuyệt đối từ `os.Executable()` |
| `SSH_ASKPASS_REQUIRE` | `force` |
| `LAZYSSH_ASKPASS_ALIAS` | alias |

Tham số thêm vào, đặt trước đích:

```
-o PreferredAuthentications=password,keyboard-interactive -o NumberOfPasswordPrompts=1
```

| Đường | Lệnh sau tính năng này (server có mật khẩu) |
|---|---|
| kết nối (`SSH`) | `ssh <auth opts> <alias>` |
| kết nối kèm tham số (`SSHWithArgs`) | `ssh <extra> <auth opts> <alias>` |
| forward (`StartForward`) | `ssh <extra> <auth opts> -N <alias>` |
| SFTP (`sftpService.Connect`) | `ssh <auth opts> -s -- <alias> sftp` |

Server không có mật khẩu: lệnh và môi trường giống hệt từng byte như trước (`cmd.Env` để `nil`).
Nếu `GetPassword` lỗi, việc khởi chạy bị huỷ với lỗi đó (không có tiến trình ssh).

Bất biến: mật khẩu không bao giờ xuất hiện trong bất kỳ tham số hay giá trị môi trường nào của bất
kỳ tiến trình nào.

## Helper (ssh → lazyssh)

Cách gọi: `$SSH_ASKPASS "<prompt>"` với `LAZYSSH_ASKPASS_ALIAS` trong môi trường. Được phát hiện
trong `main()` trước khi tạo logger hay TUI.

| Điều kiện | stdout | mã thoát |
|---|---|---|
| prompt (chuyển thường, cắt khoảng trắng phải) kết thúc bằng `password:` và chứa `<user>@<host>` của alias, và có mật khẩu đã lưu | `<password>\n` | 0 |
| prompt khác, mở được `/dev/tty` | dòng đọc từ `/dev/tty` (prompt được ghi vào đó trước) + `\n` | 0 |
| prompt khác, không mở được `/dev/tty` | không có gì | 1 |
| alias không tồn tại / không có mật khẩu / metadata hoặc `master.key` không đọc được, với prompt mật khẩu khớp | không có gì | 1 |
| `ssh -G` thất bại | xử lý như "prompt khác" | như trên |

Helper không bao giờ ghi file, không tạo `master.key`, không ghi log, và không in gì ra stdout ngoài
dòng trả lời. Lỗi in ra stderr chỉ nêu tên file có vấn đề, không bao giờ nêu bí mật.

## Các trường bí mật trong `metadata.json`

Xem data-model.md mục "Giá trị đã mã hoá". Khoá và bố cục không đổi; chỉ dạng của hai giá trị thay
đổi.

## Sync bundle

Không đổi (spec 002, version 1). `metadata` trong bundle mang `private_key` và `password` dạng
plaintext; `Import` từ chối bundle có giá trị bí mật mang tiền tố `enc:` với
`sync bundle contains encrypted values from another machine`.
