# Quickstart: kiểm tra tính năng báo lỗi kết nối

## Tự động

```sh
go test -race ./internal/core/services/ -run 'ClassifySSHError|RunSSH|RemoveKnownHost|Connect_'
go test -race ./internal/adapters/ui/ -run 'ConnectError|HostKey'
go test -race ./...
```

Phủ: phân loại theo mẫu thông báo thật của OpenSSH (bảng R3 trong [research.md](./research.md)),
trích host/file/dòng/fingerprint, `\r`, giới hạn 8 dòng; mã thoát 255 / khác / 0 với `ssh` giả trong
`PATH`; `ssh-keygen -R` trên `known_hosts` tạm; SFTP trả lỗi đã phân loại; văn bản hộp thoại được
escape.

## Thủ công / end-to-end (không đụng file thật của người dùng)

OpenSSH đọc `~/.ssh` theo thư mục home trong passwd, KHÔNG theo `$HOME`. Vì vậy mọi lần thử phải
chạy qua một `ssh` bọc trong `PATH` luôn thêm `-F <tmp>/ssh_config`, và config đó đặt
`UserKnownHostsFile <tmp>/known_hosts`, `GlobalKnownHostsFile /dev/null`, `IdentityFile <tmp>/id`,
`IdentitiesOnly yes`, `IdentityAgent none`. Lazyssh chạy với `HOME=<tmp>` để dùng
`<tmp>/.ssh/config` và `<tmp>/.lazyssh`.

1. Dựng `sshd` tạm: hai host key A/B, `sshd -f <cfg> -d -e` trên cổng cao (mỗi tiến trình nhận một
   kết nối); ghi key A vào `<tmp>/known_hosts`, chạy sshd với key B.
2. Enter vào server: hộp thoại `Host key changed` hiện `[127.0.0.1]:<port>`, `ED25519` + fingerprint
   của B, `<tmp>/known_hosts:1`; nút đang chọn là `Cancel`. Bấm Enter (Cancel): `known_hosts` không
   đổi (so `sha256sum`).
3. Lặp lại, chọn `Remove old key & reconnect`: `known_hosts.old` xuất hiện, ssh hỏi
   `Are you sure you want to continue connecting` kèm fingerprint B; gõ `yes` → vào shell; `exit 3`
   → về danh sách, không hộp thoại.
4. Cổng không có sshd → hộp thoại `Connection refused`; `Retry` kết nối lại; `Edit server` mở form.
5. Hostname `nonexistent.invalid` → `Host name not found`.
6. Đổi `IdentityFile` sang key không được cấp quyền → `Authentication failed`.
7. `m` (SFTP) với host key đổi → cùng hộp thoại host key; xoá → file manager mở sau khi trả lời
   `yes`.
8. Forward + SSH (`f`, Mode `Forward + SSH`) tới cổng đóng → hộp thoại; `Retry` dùng lại cùng
   tham số forward.

## Kết quả chạy end-to-end (2026-11-10, OpenSSH 10.6p1)

Chạy binary lazyssh thật trong pty (script Python điều khiển phím, đọc màn hình), `HOME` tạm, `ssh`/
`ssh-keygen` bọc trong `PATH` (wrapper `ssh-keygen` từ chối mọi file ngoài thư mục tạm), `sshd -d`
chạy lặp trên cổng 22422:

- Bước 2: hộp thoại đúng nội dung; Enter (mặc định) và Esc đều huỷ, `known_hosts` giữ nguyên
  (so sha256), không có `.old`.
- Bước 3: Tab + Enter → `known_hosts.old` được tạo, ssh hỏi `Are you sure you want to continue
  connecting` kèm fingerprint mới; `yes` → shell; `exit 3` → về danh sách, không hộp thoại. Trả lời
  `no` → hộp thoại `Host key not verified`. Phiên `exit` (0) → không hộp thoại, `Last SSH: just now`.
- Bước 4–6: `Connection refused`, `Host name not found`, `Authentication failed` (user không tồn
  tại) đúng nguyên nhân; Enter = Close; Retry kết nối lại; Edit server mở form Edit Server.
- Bước 7: SFTP với host key đổi → cùng hộp thoại; xoá → `yes` → file manager mở. sshd không có
  `Subsystem sftp` → hộp thoại `SSH connection failed` với nguyên văn
  `subsystem request failed on channel 0`.
- Bước 8: Forward + SSH tới cổng đóng → hộp thoại; Retry chạy lại với cùng `-L 18080:localhost:80`
  (log).
- Xoá key thất bại (thư mục chứa `known_hosts` chỉ đọc) → hộp thoại `Could not remove the old host
  key` với `mkstemp: Permission denied`, không kết nối lại.
- Chưa kiểm được: askpass (mật khẩu đã lưu) với stderr qua pipe — `sshd` không root không xác thực
  mật khẩu được; chỉ suy luận từ code (helper mở tty riêng).
