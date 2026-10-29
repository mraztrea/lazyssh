# Quickstart: kiểm tra tính năng lọc file SFTP

## Tự động

```sh
go test -race ./internal/adapters/ui/ -run 'Filter|FilePane'
go test -race ./...
```

Phủ: `filterEntries`, `SetFilter` (hàng, `../`, `Selected()`, title), `Load` xoá/giữ filter, `step`.

## Thủ công (phần nối phím tcell)

Chuẩn bị: `make build` (hoặc `go build ./cmd/...`), một server SFTP được, thư mục remote có vài file
và thư mục tên hoa/thường lẫn lộn (ví dụ `app.LOG`, `logs/`, `readme.md`).

1. Chọn server, bấm `m`. Dòng hints có `/ Filter`.
2. Tab sang pane Remote, bấm `/`: dòng hints thành `Filter: `, con trỏ trong ô nhập.
3. Gõ `log`: Remote chỉ còn `../`, `logs/`, `app.LOG`; title thành ` Remote: <cwd> (<alias>) [/log] `;
   Local không đổi.
4. Gõ thêm `u`, `d`, `q`: chúng thành text (`logudq`), không tải/đóng gì; bảng chỉ còn `../`.
   Xoá về `log`.
5. Bấm Xuống: focus về bảng Remote, con trỏ ở `logs/` (entry đầu sau `../`), dòng hints trở lại,
   title vẫn có `[/log]`. Bấm `d` trên `app.LOG`: đúng file đó được tải về; sau khi xong filter vẫn còn.
6. Bấm `/` lần nữa: ô nhập mở với sẵn `log`. Bấm Lên: focus về bảng, con trỏ ở `../`.
7. Bấm `/`, rồi Tab: focus về bảng, filter giữ, con trỏ không đổi.
8. Bấm `/`, rồi Esc: filter bị xoá, Remote hiện đầy đủ, title hết `[/log]`, file manager KHÔNG đóng.
9. Đặt filter `log`, Enter vào `logs/`: thư mục mới hiện đầy đủ, title không còn `[/...]`.
10. Đặt filter trên Local, Tab sang Remote: filter Local vẫn hiện trên title Local.
11. Không dùng filter: mọi phím cũ (Tab, Enter, h, u, d, n, r, D, R, g, q, Esc) hoạt động như trước.
