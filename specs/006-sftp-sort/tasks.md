---

description: "Danh sách task cho Sắp xếp file trong trình quản lý file SFTP"
---

# Tasks: Sắp xếp file trong trình quản lý file SFTP

**Input**: Tài liệu thiết kế trong `/specs/006-sftp-sort/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Bắt buộc. Hiến pháp II: viết test, chạy và thấy nó FAIL, hiện thực, thấy PASS, commit.
Test dùng `list` giả như `TestFilePane_LoadRendersParentRow`, không đụng filesystem. Phần nối phím
tcell (T011–T012) được kiểm thủ công theo quickstart.md.

**Organization**: Mỗi phase ứng với một task trong plan.md ("Thiết kế theo task", T1–T4); mỗi phase
kết thúc bằng một commit với message kết thúc bằng dòng `Co-Authored-By` của phiên. US2 (định dạng
thời gian) độc lập và nhỏ nhất nên đi trước; logic sort của `filePane` (plan T2) phục vụ US1 và US3
và đi trước phần nối phím (plan T3).

## Format: `[ID] [P?] [Story] Mô tả`

- **[P]**: Có thể chạy song song (file khác nhau, không phụ thuộc)
- **[Story]**: US1 sắp theo Name/Modified, US2 thời gian sửa đầy đủ, US3 chế độ bền trong phiên

## Phase 1: Setup

Không có task setup: module, công cụ và test đã có; không thêm dependency.

## Phase 2: Nền tảng (Foundational)

Không có: mọi thay đổi nằm trong hai file UI có sẵn.

---

## Phase 3: User Story 2 — Thời gian sửa đầy đủ (Priority: P1) — plan T1

**Mục tiêu**: Cột Modified hiện `YYYY-MM-DD HH:MM:SS` giờ local.

**Kiểm thử độc lập**: `go test -race ./internal/adapters/ui/ -run ModifiedColumn -v`

- [X] T001 [US2] Viết test thất bại `TestFilePane_ModifiedColumn` trong internal/adapters/ui/sftp_pane_test.go: list giả một file `ModTime = time.Date(2026, 10, 29, 14, 5, 9, 0, time.Local)`, `Load("/srv")` → `GetCell(2, 2).Text == " 2026-10-29 14:05:09"`; chạy và xác nhận FAIL (đang là ` 10-29`)
- [X] T002 [US2] Trong `render()` của internal/adapters/ui/sftp_pane.go đổi layout `"01-02"` thành `"2006-01-02 15:04:05"`; chạy `go test -race ./...`, `gofmt -l .`, `go vet ./...`; xác nhận PASS
- [X] T003 Commit `feat: show full modified time in sftp file pane` (sftp_pane.go, sftp_pane_test.go, tasks.md)

---

## Phase 4: User Story 1 + 3 — Sắp xếp trong `filePane` (Priority: P1/P2) — plan T2

**Mục tiêu**: Pane sắp theo Name hoặc Modified, ↑/↓, thư mục trước, tie-break theo tên tăng dần;
title hiện chế độ; đổi chế độ giữ con trỏ và filter; nạp lại/lọc giữ chế độ.

**Kiểm thử độc lập**: `go test -race ./internal/adapters/ui/ -run 'Sort|FilePane' -v`

- [X] T004 [US1] Trong internal/adapters/ui/sftp_pane_test.go: sửa `TestSortEntries_DirsFirstCaseInsensitive` gọi `sortEntries(e, paneSort{})`; thêm `TestPaneSort_String` (`paneSort{}` → `Name ↑`, `{desc: true}` → `Name ↓`, `{byTime: true}` → `Modified ↑`, `{true, true}` → `Modified ↓`) và `TestSortEntries_Modes` (thư mục `d1` cũ, `D2` mới; file `b` t1, `a` t2, `C` t2, `e` t3 với t1<t2<t3: `Name ↓` → `D2, d1, e, C, b, a`; `Modified ↑` → `d1, D2, b, a, C, e`; `Modified ↓` → `D2, d1, e, a, C, b` — cùng t2 thì `a` trước `C` ở cả hai chiều)
- [X] T005 [US1] Thêm `TestFilePane_SetSort` vào cùng file: list giả như T004; `Load`, `Select(4, 0)`, `SetSort(paneSort{byTime: true, desc: true})` → các hàng 2.. theo `Modified ↓`, `GetSelection()` vẫn là 4, `GetTitle()` chứa `[Modified ↓`; `SetFilter("c")` → chỉ `C` và vẫn có `[Modified ↓`
- [X] T006 [US3] Thêm `TestFilePane_LoadKeepsSort` vào cùng file: sau `SetSort(paneSort{byTime: true, desc: true})`, `Load` thư mục khác → `p.sort` giữ nguyên và hàng 2.. vẫn theo `Modified ↓`
- [X] T007 Chạy `go test ./internal/adapters/ui/ -run 'Sort|FilePane' -v`; xác nhận FAIL (chưa có `paneSort`, `SetSort`, chữ ký `sortEntries` mới)
- [X] T008 [US1] Trong internal/adapters/ui/sftp_pane.go: `type paneSort struct{ byTime, desc bool }` (comment: giá trị rỗng là `Name ↑`) với `String()` trả `Name`/`Modified` + ` ↑`/` ↓`; `sortEntries(e []fileEntry, s paneSort)` theo data-model.md: "Thư mục trước file", "`byTime` và mtime khác nhau → mtime theo chiều", "`byTime` và mtime bằng nhau → tên không phân biệt hoa thường, tăng dần", "Theo Name → tên không phân biệt hoa thường, theo chiều"; `filePane` thêm `sort paneSort`; `Load` bỏ lời gọi `sortEntries`; `render()` gọi `sortEntries(p.all, p.sort)` trước `filterEntries`, title thêm `tview.Escape(" [" + p.sort.String() + "]")` trước phần filter; `SetSort(s paneSort)` nhớ hàng đang chọn, gán, `render()`, `Select` lại hàng đó
- [X] T009 [US3] Xác nhận `TestFilePane_LoadKeepsSort` và các test filter cũ PASS (sort nằm trong `render()` nên `Load`/`SetFilter` dùng chế độ hiện tại); chạy `go test -race ./...`, `gofmt -l .`, `go vet ./...`
- [X] T010 Commit `feat: sort sftp file pane by name or modified time` (sftp_pane.go, sftp_pane_test.go, tasks.md)

---

## Phase 5: User Story 1 — Phím `s`/`S` trong file manager (Priority: P1) — plan T3

**Mục tiêu**: `s` đổi trường, `S` đảo chiều của pane đang active; status báo `Sort: <mode>`; hints
có `s Sort`, `S Reverse`.

**Kiểm thử độc lập**: quickstart.md mục "Thủ công" bước 1–8.

- [ ] T011 [US1] Trong internal/adapters/ui/sftp_view.go: method `sortBy(s paneSort)` (`fm.active.SetSort(s)`, `fm.info("Sort: " + s.String())`); map rune của `handleKey` thêm `'s'` → `sortBy(paneSort{!cur.byTime, cur.desc})`, `'S'` → `sortBy(paneSort{cur.byTime, !cur.desc})` với `cur := fm.active.sort`; `fileManagerHints` thêm `[white]s[-] Sort  • [white]S[-] Reverse`
- [ ] T012 Chạy `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test -race ./...`; xác nhận PASS; kiểm phím bằng test tạm gửi `s`/`S` vào `handleKey` (xoá sau khi chạy) hoặc các bước thủ công trong quickstart.md nếu có terminal
- [ ] T013 Commit `feat: add s/S sort keys to sftp file manager` (sftp_view.go, tasks.md)

---

## Phase 6: Hoàn thiện — plan T4

- [ ] T014 [P] Cập nhật README.md: thêm hàng `s` (`Toggle sort field (Name/Modified)`) và `S` (`Reverse sort order`) vào bảng "In SFTP File Manager" và một gạch đầu dòng ở mục "SFTP File Manager"
- [ ] T015 Commit `docs: document sftp file sort` (README.md, specs/006-sftp-sort/tasks.md)

---

## Phụ thuộc & thứ tự

- Phase 3 độc lập; Phase 4 → Phase 5 (T011 gọi `SetSort`, đọc `sort`) → Phase 6.
- Phase 3 và Phase 4 cùng sửa `sftp_pane.go`/`sftp_pane_test.go` nên tuần tự.
- T014 độc lập về file, có thể làm song song với Phase 5.

## Ví dụ chạy song song

```text
Phase 5 (sftp_view.go) ‖ T014 (README.md)
```

## Chiến lược hiện thực

- **MVP**: Phase 3 + Phase 4 + Phase 5 (US1 + US2): thấy đủ thời gian và sắp theo Modified.
- Tăng dần: US3 đi kèm Phase 4 (không cần code riêng), rồi tài liệu. Mỗi phase để
  `go test -race ./...` xanh.
