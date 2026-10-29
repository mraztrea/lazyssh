# Quickstart: Kiểm chứng xác thực bằng mật khẩu và bí mật được mã hoá

## Tự động

```sh
make test                                                    # mọi package, -race
go test ./internal/secretbox/ -v                             # seal/open, tamper, plaintext cũ, file key
go test ./internal/adapters/data/ssh_config_file/ -run 'Metadata|Password|Export|Import' -v
go test ./internal/core/services/ -run 'Askpass|ParseSSHG|PasswordAuth|SSHCmd|SFTP|Validate' -v
go test ./internal/adapters/ui/ -run 'AuthMethod|Details' -v
make build                                                   # fmt, vet, golangci-lint, binary
```

Các bảo đảm được ghim bằng test: `TestPasswordAuth_SecretNotInArgsOrEnv`,
`TestMetadata_SecretsEncryptedOnDisk`, `TestMetadata_MissingMasterKeyFailsWithoutWrite`,
`TestAskpass_OtherHostPromptNotAnswered`, `TestExportImport_DifferentMasterKeys`.

## Thủ công (HOME tạm, sshd cục bộ bật xác thực mật khẩu, ví dụ container ở cổng 2222)

```sh
export H=$(mktemp -d) && mkdir -p $H/.ssh
printf 'Host pw\n  HostName 127.0.0.1\n  Port 2222\n  User tester\n' > $H/.ssh/config
HOME=$H ./bin/lazyssh          # chạy trong tmux để điều khiển
```

1. `e` trên `pw` → tab Authentication → `Auth method: password`; chỉ `Password:` được bật. Gõ
   mật khẩu (hiện dạng `*`), lưu. Details pane hiện `(password)`, không bao giờ hiện giá trị.
2. `grep -c enc:v1: $H/.lazyssh/metadata.json` → `1`; grep mật khẩu → `0`;
   `stat -c %a $H/.lazyssh/master.key` → `600`.
3. Enter trên `pw`: lần đầu, câu hỏi host key hiện trên terminal (được chuyển tiếp); trả lời
   `yes`; shell mở ra mà không hỏi mật khẩu. `m` (file manager) trên `pw` mở cả hai pane mà không
   hỏi gì.
4. Khi đang kết nối, từ shell khác: `ps eww -u $USER | grep -c '<password>'` → `0`.
5. Sửa `pw`, đặt mật khẩu sai, kết nối: một lần `Permission denied`, không lặp thử lại.
6. Di chuyển `$H/.lazyssh/master.key` đi chỗ khác rồi khởi động lại: `pw` vẫn hiện, details pane
   ghi `(locked: … master.key …)`, server khác bình thường; Enter trên `pw` → lỗi đỏ nêu tên
   `master.key`; pin `pw` rồi so `sha256sum` của giá trị `enc:v1:` trước/sau: không đổi, không có
   `master.key` mới. Trả lại file: `pw` hết khoá.
7. Plaintext cũ: ghi `{"pw":{"password":"plain"}}` vào `metadata.json`, khởi động, pin `pw` (`p`):
   file giờ chỉ còn `enc:v1:`.
8. Sync: `y` → Push tới `/tmp/s.enc` (password `x`), xoá `$H/.lazyssh/master.key` và
   `metadata.json`, Pull: `pw` kết nối lại được; có `master.key` mới.
