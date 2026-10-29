# Data model: Lọc file trong trình quản lý file SFTP

Chỉ là trạng thái trong bộ nhớ của UI; không có gì được lưu xuống đĩa.

## `filePane` (sửa)

| Trường | Kiểu | Ý nghĩa |
|---|---|---|
| `all` | `[]fileEntry` | MỚI. Toàn bộ entry của `cwd`, đã sort (thư mục trước, không phân biệt hoa thường). |
| `filter` | `string` | MỚI. Chuỗi lọc của pane; rỗng = không lọc. |
| `entries` | `[]fileEntry` | ĐỔI NGHĨA. `filterEntries(all, filter)`: các entry đang hiển thị, hàng `i+2` ↔ `entries[i]`. |

Quy tắc:

- Entry khớp khi `strings.ToLower(Name)` chứa `strings.ToLower(filter)`; thứ tự giữ nguyên.
- Hàng 0 là header, hàng 1 luôn là `../`, không bị lọc.

## Vòng đời của `filter`

```text
"" --(gõ trong ô filter)--> "q" --(gõ tiếp / xoá)--> "q'"
"q" --(Esc trong ô filter | xoá hết chuỗi)--> ""
"q" --(Load thư mục khác cwd)--> ""
"q" --(Load cùng cwd: R, reload sau thao tác)--> "q"
"q" --(Tab chuyển pane)--> "q" (pane kia có filter riêng)
```

## `fileManager` (sửa)

| Trường | Kiểu | Ý nghĩa |
|---|---|---|
| `filterInput` | `*tview.InputField` | MỚI. Ô `Filter: ` thay chỗ `status` trong `layout` khi đang nhập. |
