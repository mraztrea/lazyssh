# Quickstart: kiểm tra tính năng Home Dir

## Tự động

```sh
go test -race ./internal/core/services/ -run 'HomeDir|ResolveRemoteDir|SSHArgs|ValidateServer'
go test -race ./internal/adapters/data/ssh_config_file/ -run 'HomeDir'
go test -race ./internal/adapters/ui/ -run 'ServerDetails'
go test -race ./...
```

Phủ: sinh lệnh remote (quoting, `~`, RemoteCommand, `none`); resolve đường dẫn SFTP; argv ssh có/không
Home Dir; từ chối xuống dòng; round-trip/rename metadata; pane chi tiết.

## Thủ công (kết nối thật)

Chuẩn bị: `make build`, một server SSH/SFTP được, trên server có `/tmp/hd test` (có khoảng trắng) và
`~/proj`.

1. Chọn server, `e` (Edit): ô `Home Dir:` ở tab cơ bản sau `Tags:` với placeholder
   `e.g. /var/www/app`; nhập `/tmp/hd test`, Save. Pane chi tiết hiện `Home Dir: /tmp/hd test`.
   `grep -c home_dir ~/.lazyssh/metadata.json` → 1; `grep -i "home" ~/.ssh/config` không có gì mới.
2. Enter để SSH: `pwd` in `/tmp/hd test`; `echo $0` là login shell quen thuộc; `exit` quay lại lazyssh.
3. `m` (SFTP): pane Remote tiêu đề/đường dẫn là `/tmp/hd test`; pane Local vẫn ở home cục bộ.
4. Sửa Home Dir thành `~/proj`: SSH `pwd` → `/home/<user>/proj`; SFTP pane Remote cũng vậy.
5. Sửa Home Dir thành `/no/such/dir`: SSH in `cd: /no/such/dir: No such file or directory` rồi vẫn
   có dấu nhắc tại home; SFTP mở tại home remote và status đỏ `Home dir not found: /no/such/dir`.
6. Thêm `RemoteCommand tmux attach || tmux new` vào `~/.ssh/config` cho host đó, Home Dir `~/proj`:
   Enter → vào tmux, `pwd` trong tmux (cửa sổ mới) là `~/proj`; không có lỗi
   `Cannot execute command-line and remote command.`
7. Xoá trắng Home Dir, Save: `grep -c home_dir ~/.lazyssh/metadata.json` → 0; SSH và SFTP về thư mục
   mặc định như trước; `c` copy lệnh ssh không chứa `cd`.
8. Port forward (`f`) với Home Dir đặt lại: chế độ tương tác mở shell tại Home Dir; chế độ nền chạy
   bình thường (không lệnh remote).
9. Đổi alias trong form Edit: Home Dir vẫn còn sau khi lưu; `y` Push rồi Pull trên máy khác: Home
   Dir đi theo.
