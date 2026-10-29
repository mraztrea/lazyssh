# Quickstart: kiểm tra tính năng sắp xếp file SFTP

## Tự động

```sh
go test -race ./internal/adapters/ui/ -run 'Sort|FilePane'
go test -race ./...
```

Phủ: `sortEntries` bốn chế độ và tie-break, `paneSort.String`, `SetSort` (thứ tự, title, con trỏ,
filter), `Load` giữ chế độ, định dạng cột Modified.

## Thủ công (phần nối phím tcell)

Chuẩn bị: `make build` (hoặc `go build ./cmd/...`), một server SFTP được, thư mục remote có vài thư
mục và file với thời gian sửa khác nhau (ví dụ `touch -d '2024-01-01' old.txt; touch new.txt`).

1. Chọn server, bấm `m`. Dòng hints có `s Sort` và `S Reverse`; title hai pane kết thúc bằng
   ` [Name ↑] `; cột Modified dạng `2026-10-29 14:05:09`.
2. Tab sang Remote, đưa con trỏ xuống hàng 3, bấm `s`: title thành ` [Modified ↑] `, dòng status
   `Sort: Modified ↑`, thư mục vẫn ở trên, file cũ nhất đứng đầu nhóm file, con trỏ vẫn ở hàng 3.
   Pane Local không đổi.
3. Bấm `S`: ` [Modified ↓] `, file mới nhất đứng đầu nhóm file.
4. Bấm `s`: ` [Name ↓] `, tên Z→A. Bấm `S`: về ` [Name ↑] `.
5. Đặt lại `Modified ↓`, Enter vào một thư mục con: vẫn ` [Modified ↓] ` và đúng thứ tự. Bấm `R`:
   giữ nguyên.
6. Bấm `/`, gõ `s`: chữ `s` vào ô filter (không đổi chế độ); danh sách lọc giữ thứ tự `Modified ↓`;
   title có cả ` [Modified ↓]` và ` [/s]`. Bấm Esc.
7. Đóng (`q`) rồi mở lại (`m`): cả hai pane về ` [Name ↑] `.
8. Mọi phím cũ (Tab, Enter, h, u, d, n, r, D, R, g, /, q, Esc) hoạt động như trước.
