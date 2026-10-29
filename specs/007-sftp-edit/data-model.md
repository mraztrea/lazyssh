# Data model: Xem/sửa file bằng editor trong trình quản lý file SFTP

Không có dữ liệu bền nào được lưu; chỉ có bản sao tạm sống trong một lần sửa remote và hai hàm mới
ở `internal/core/services/sftp_edit.go`.

## Hàm mới

| Hàm | Vào | Ra | Ghi chú |
|---|---|---|---|
| `EditorCommand(env string) []string` | giá trị `$EDITOR` | argv của editor | `strings.Fields(env)`, rỗng → `["vi"]`. |
| `EditRemote(c *sftp.Client, remotePath string, run func(localPath string) error) (changed bool, err error)` | client SFTP, đường dẫn file remote, hàm chạy editor | có ghi lại remote hay không, lỗi | Xem vòng đời dưới. |

## Bản sao tạm

| Thuộc tính | Giá trị |
|---|---|
| Thư mục | `$TMPDIR/lazyssh-edit-<ngẫu nhiên>/`, mode 0700, riêng cho mỗi lần sửa |
| File | `<thư mục>/<tên file remote>`, mode 0600 |
| Dấu vân tay | SHA-256 nội dung, tính ngay sau khi tải xuống và sau khi `run` trả về |

## Vòng đời của một lần sửa remote

```text
tạo thư mục tạm --lỗi--> (false, err)
  └─ tải xuống --lỗi--> xoá thư mục, (false, err)
       └─ hash trước → run(local) --lỗi--> xoá thư mục, (false, err)
            └─ hash sau == hash trước --> xoá thư mục, (false, nil)      "No changes"
            └─ hash sau != hash trước --> ghi đè remote
                 ├─ ok   --> xoá thư mục, (true, nil)                     "Saved <name>"
                 └─ lỗi  --> GIỮ thư mục, (true, err kèm đường dẫn bản sao) "Edit failed: ..."
```

## `fileManager` (sửa)

Không thêm trường. Phím `e` dùng `active`, `busy`, `client`, `reload`, `after` có sẵn.
