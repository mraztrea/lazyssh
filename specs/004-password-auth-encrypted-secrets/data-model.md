# Mô hình dữ liệu: Xác thực bằng mật khẩu và mã hoá bí mật khi lưu trữ

## Server (`internal/core/domain/server.go`) — mở rộng

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `PrivateKey` | string | có sẵn (spec 001); plaintext trong bộ nhớ |
| `Password` | string | MỚI; plaintext trong bộ nhớ, `""` khi không có; không chứa `\n`/`\r` (`validateServer`) |

Suy ra, không lưu: phương thức xác thực = `password` nếu `Password != ""`, nếu không thì
`embedded key` nếu `PrivateKey != ""`, nếu không thì `identity file`. Sau khi lưu qua form, tối đa
một trong hai khác rỗng (FR-003); `metadata.json` sửa tay có cả hai thì kết nối bằng mật khẩu
(FR-011).

## ServerMetadata (`internal/adapters/data/ssh_config_file/metadata_manager.go`) — mở rộng

| Trường | JSON | Trong bộ nhớ | Trên đĩa |
|---|---|---|---|
| `Tags`, `LastSeen`, `PinnedAt`, `SSHCount` | không đổi | không đổi | không đổi |
| `PrivateKey` | `private_key,omitempty` | plaintext | `enc:v1:<base64>`; chấp nhận plaintext cũ khi đọc |
| `Password` | `password,omitempty` | plaintext | `enc:v1:<base64>`; chấp nhận plaintext cũ khi đọc |

`loadAll` mở các giá trị đã mã hoá (key nạp lười); `saveAll` và `Import` mã hoá mọi bí mật không
rỗng (tạo key nếu thiếu). `mergeMetadata` chép `Password` vào `domain.Server`; `updateServer` chép
`server.Password` vào mục (đổi tên alias thì nó đi cùng mục).

## Giá trị đã mã hoá (`internal/secretbox`)

```
enc:v1:<base64.StdEncoding( nonce[12] || ciphertext+tag AES-256-GCM )>
```

| Đầu vào | Kết quả `Open` |
|---|---|
| không có tiền tố `enc:` | trả nguyên (plaintext cũ) |
| `enc:v1:` + hợp lệ | plaintext |
| `enc:<khác>:` | `ErrUnknownFormat` |
| base64 sai / quá ngắn / xác thực GCM thất bại | `ErrDecrypt` (thông báo không bao giờ chứa giá trị) |

## Master key

| Thuộc tính | Giá trị |
|---|---|
| Đường dẫn | `filepath.Dir(metadataPath)/master.key` = `~/.lazyssh/master.key` |
| Nội dung | 32 byte ngẫu nhiên (`crypto/rand`) |
| Mode | file 0600, thư mục 0700 |
| Tạo khi | chỉ bởi thao tác ghi phải mã hoá một giá trị, qua `O_CREATE|O_EXCL` |
| Lỗi | thiếu → `master key <path>: file does not exist`; kích thước ≠ 32 → `master key <path> is corrupt` |
| Sync | không bao giờ |

## ServerFormData (`internal/adapters/ui/server_form.go`) — mở rộng, chỉ ở UI

| Trường | Ghi chú |
|---|---|
| `AuthMethod` | `identity file` / `embedded key` / `password`; giá trị ban đầu suy ra như trên |
| `Password` | lấy từ trường `Password:` có che ký tự |

`applyAuthMethod(*ServerFormData) error`: `password` → `PrivateKey = ""`, `PrivateKeyImportPath =
""`, `Password` rỗng → `password is required`; `embedded key` → `Password = ""`; `identity file` →
`Password = ""`, `PrivateKey = ""`, `PrivateKeyImportPath = ""`. Được gọi trong `handleSave` trước
`resolvePrivateKey`.

## Yêu cầu askpass (hợp đồng tiến trình, xem contracts/askpass.md)

| Đầu vào | Nguồn |
|---|---|
| alias | env `LAZYSSH_ASKPASS_ALIAS` |
| prompt | `os.Args[1]` (rỗng nếu không có) |
| mật khẩu | `ports.ServerRepository.GetPassword(alias)` |
| `user@host` đích | `ssh -G <alias>`: `user`, `hostkeyalias` hoặc `hostname` |
