# Data model: Sắp xếp file trong trình quản lý file SFTP

Chỉ là trạng thái trong bộ nhớ của UI; không có gì được lưu xuống đĩa.

## `paneSort` (mới)

| Trường | Kiểu | Ý nghĩa |
|---|---|---|
| `byTime` | `bool` | `false` = theo Name, `true` = theo Modified (mtime). |
| `desc` | `bool` | `false` = ↑ (tăng dần: A→Z, cũ→mới), `true` = ↓ (giảm dần). |

`String()`: `Name ↑` (giá trị rỗng, mặc định), `Name ↓`, `Modified ↑`, `Modified ↓`.

Quy tắc thứ tự (`sortEntries`):

1. Thư mục trước file.
2. `byTime` và mtime khác nhau → mtime theo chiều.
3. `byTime` và mtime bằng nhau → tên không phân biệt hoa thường, tăng dần.
4. Theo Name → tên không phân biệt hoa thường, theo chiều.

## `filePane` (sửa)

| Trường | Kiểu | Ý nghĩa |
|---|---|---|
| `sort` | `paneSort` | MỚI. Chế độ sort của pane; `render()` áp dụng lên `all` trước khi lọc. |
| `all` | `[]fileEntry` | ĐỔI NGHĨA. Toàn bộ entry của `cwd`, đã sort theo `sort` (trước đây luôn theo tên). |

## Vòng đời của `sort`

```text
{Name ↑} --(s)--> {Modified ↑} --(S)--> {Modified ↓} --(s)--> {Name ↓} --(S)--> {Name ↑}
mọi chế độ --(Load bất kỳ thư mục | SetFilter | Tab chuyển pane)--> giữ nguyên
đóng file manager --> mất; mở lại là {Name ↑}
```
