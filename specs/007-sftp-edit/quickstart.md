# Quickstart: kiểm tra tính năng sửa file SFTP

## Tự động

```sh
go test -race ./internal/core/services/ -run 'EditorCommand|EditRemote'
go test -race ./...
```

Phủ: chọn editor từ `EDITOR`; `EditRemote` khi có đổi, không đổi, editor lỗi, tải xuống lỗi, tải lên
lỗi; quyền 0600 của bản sao; giữ mode file remote; dọn thư mục tạm.

## Thủ công (phần nối phím tcell và editor thật)

Chuẩn bị: `make build` (hoặc `go build ./cmd/...`), một server SFTP được, một file remote có thể ghi
(ví dụ `~/edit-me.txt` chứa `hello`, `chmod 640`).

`EDITOR` được tách theo khoảng trắng, không hiểu dấu nháy như shell, nên editor giả là một script:

```sh
printf '#!/bin/sh\necho x >> "$1"\n' > /tmp/fake-editor && chmod +x /tmp/fake-editor
```

1. `EDITOR=/tmp/fake-editor ./lazyssh`, chọn server, bấm `m`. Dòng hints có `e Edit`.
2. Tab sang Remote, chọn `edit-me.txt`, bấm `e`: màn hình chớp, quay lại; status xanh
   `Saved edit-me.txt`; trên server file có thêm dòng `x`, mode vẫn `640`. `ls -d /tmp/lazyssh-edit-*`
   không còn thư mục nào.
3. Thoát, chạy lại với `EDITOR=true`: `e` trên `edit-me.txt` → `No changes`; file không đổi.
4. `EDITOR=false`: `e` → status đỏ `Edit failed: exit status 1`; file không đổi.
5. `EDITOR=no-such-editor`: `e` → status đỏ `Edit failed: exec: "no-such-editor": executable file
   not found in $PATH`.
6. Bấm `e` trên một thư mục hoặc hàng `../`: không có gì xảy ra.
7. Pane Local, `EDITOR=/tmp/fake-editor`: `e` trên một file → status `Edited <name>`, cột Size tăng.
8. Không đặt `EDITOR` (`env -u EDITOR ./lazyssh`): `e` trên file Remote mở `vi` trên bản sao; sửa,
   `:wq` → `Saved <name>`; mở lại, `:q` → `No changes`.
9. Tải lên lỗi: với `EDITOR=vi`, mở file Remote; trong lúc vi đang mở, ở terminal khác
   `chmod 400` file đó trên server (với user không phải root); sửa, `:wq` → status đỏ
   `Edit failed: ... (edited copy kept at /tmp/lazyssh-edit-.../<name>)`; bản sao còn đó với nội
   dung đã sửa.
10. Bắt đầu tải xuống một thư mục lớn (`d`), trong lúc chạy bấm `e` ở pane Remote → status đỏ
    `A transfer is already running`.
11. Bấm `/`, gõ `e`: chữ `e` vào ô filter, không mở editor. Mọi phím cũ hoạt động như trước.
