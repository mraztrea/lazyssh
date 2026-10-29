---

description: "Danh sách task cho Lọc file trong trình quản lý file SFTP"
---

# Tasks: Lọc file trong trình quản lý file SFTP

**Input**: Tài liệu thiết kế trong `/specs/005-sftp-filter/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Bắt buộc. Hiến pháp II: viết test, chạy và thấy nó FAIL, hiện thực, thấy PASS, commit.
Test dùng `list` giả như `TestFilePane_LoadRendersParentRow`, không đụng filesystem. Phần nối phím
tcell (T007–T009) được kiểm thủ công theo quickstart.md.

**Organization**: Mỗi phase ứng với một task trong plan.md ("Thiết kế theo task", T1–T3); mỗi phase
kết thúc bằng một commit với message kết thúc bằng dòng `Co-Authored-By` của phiên. Logic của
`filePane` (plan T1) phục vụ cả US1 và US2 nên đi trước phần nối phím (plan T2) mà US1–US3 cần.

## Format: `[ID] [P?] [Story] Mô tả`

- **[P]**: Có thể chạy song song (file khác nhau, không phụ thuộc)
- **[Story]**: US1 lọc theo tên, US2 rời ô filter để thao tác, US3 huỷ/sửa filter

## Phase 1: Setup

Không có task setup: module, công cụ và test đã có; không thêm dependency.

## Phase 2: Nền tảng (Foundational)

Không có: mọi thay đổi nằm trong hai file UI có sẵn.

---

## Phase 3: User Story 1 + 2 — Lọc trong `filePane` (Priority: P1) — plan T1

**Mục tiêu**: Pane lọc được danh sách đã nạp theo substring không phân biệt hoa thường, giữ `../`,
hiện filter trên title, xoá filter khi đổi thư mục, di chuyển con trỏ có kẹp.

**Kiểm thử độc lập**: `go test -race ./internal/adapters/ui/ -run 'Filter|FilePane' -v`

- [X] T001 [US1] Viết test thất bại trong internal/adapters/ui/sftp_pane_test.go: `TestFilterEntries` (q rỗng → trả nguyên; `LOG` khớp `app.log` và `Logs`; không khớp → rỗng; thứ tự giữ nguyên), `TestFilePane_SetFilter` (list giả `logs/`, `app.LOG`, `readme.md`; `SetFilter("log")` → `GetRowCount() == 4`, hàng 1 là `../`, hàng được chọn là 1, `Select(3,0)` → `Selected()` là `app.LOG`, `GetTitle()` chứa `[/log`; `SetFilter("zzz")` → `GetRowCount() == 2`; `SetFilter("")` → đủ 5 hàng, title không còn `[/`), `TestFilePane_LoadClearsFilterOnDirChange` (`Load` cùng `cwd` giữ `filter` và vẫn lọc; `Load` thư mục khác → `filter == ""`)
- [X] T002 [US2] Thêm vào cùng file `TestFilePane_Step` (sau `SetFilter("log")`, con trỏ ở 1: `step(1)` → 2, `step(1)` → 3, `step(1)` → 3 (kẹp cuối), `step(-5)` → 1 (không lên header); không khớp: `step(1)` → 1)
- [X] T003 Chạy `go test ./internal/adapters/ui/ -run 'Filter|FilePane' -v`; xác nhận FAIL (chưa có `filterEntries`, `SetFilter`, `step`)
- [X] T004 [US1] Trong internal/adapters/ui/sftp_pane.go: hàm `filterEntries(e []fileEntry, q string) []fileEntry` ("Entry khớp khi `strings.ToLower(Name)` chứa `strings.ToLower(filter)`; thứ tự giữ nguyên"); `filePane` thêm `all []fileEntry` và `filter string` (comment `entries` = danh sách đã lọc, hàng `i+2`); `Load` giữ nhánh lỗi cũ, `if dir != p.cwd { p.filter = "" }`, gán `p.all, p.cwd` rồi gọi `p.render()`; `render()` chứa phần dựng bảng cũ, lặp trên `p.entries = filterEntries(p.all, p.filter)`, title thêm `tview.Escape(" [/"+p.filter+"]")` khi filter khác rỗng, `Select(1,0)`, `ScrollToBeginning()`; `SetFilter(q string)` = gán + `render()`; `step(d int)` chọn `min(max(row+d, 1), GetRowCount()-1)`
- [X] T005 Chạy `make test` (`go test -race ./...`), `gofmt -l .`, `go vet ./...`; xác nhận PASS
- [X] T006 Commit `feat: filter entries in sftp file pane` (sftp_pane.go, sftp_pane_test.go)

---

## Phase 4: User Story 1–3 — Ô filter trong file manager (Priority: P1/P2) — plan T2

**Mục tiêu**: `/` mở ô `Filter: ` thay dòng hints; gõ là lọc pane active; Lên/Xuống/Enter/Tab rời ô
giữ filter; Esc xoá filter; `/` lần nữa sửa chuỗi cũ.

**Kiểm thử độc lập**: quickstart.md mục "Thủ công" bước 1–11.

- [ ] T007 [US1] Trong internal/adapters/ui/sftp_view.go: trường `filterInput *tview.InputField` trên `fileManager`; tạo trong `showFileManager` (label `Filter: `, `SetBackgroundColor`/`SetFieldBackgroundColor(tcell.Color235)` như `status`), `SetChangedFunc(func(q string) { fm.active.SetFilter(q) })`; method `startFilter()` (`SetText(fm.active.filter)`, `layout.RemoveItem(status)`, `layout.AddItem(filterInput, 1, 0, false)`, `app.SetFocus(filterInput)`); `handleKey` mở đầu bằng `if fm.filterInput.HasFocus() { return ev }` và map rune thêm `'/': fm.startFilter`; `fileManagerHints` thêm `[white]/[-] Filter`
- [ ] T008 [US2] Cùng file: method `endFilter(d int)` (`RemoveItem(filterInput)`, `AddItem(status, 1, 0, false)`, `app.SetFocus(fm.active)`, `if d != 0 { fm.active.step(d) }`); `filterInput.SetInputCapture`: `KeyDown`/`KeyEnter` → `endFilter(1)`, `KeyUp` → `endFilter(-1)`, `KeyTab` → `endFilter(0)`, trả `nil`; phím khác trả `ev`
- [ ] T009 [US3] Cùng capture: `KeyEscape` → `filterInput.SetText("")` (changed func hiện lại đầy đủ) rồi `endFilter(0)`, trả `nil`
- [ ] T010 Chạy `go build ./...`, `go vet ./...`, `gofmt -l .`, `make test`; xác nhận PASS; chạy các bước thủ công trong quickstart.md nếu có terminal
- [ ] T011 Commit `feat: add / filter input to sftp file manager` (sftp_view.go)

---

## Phase 5: Hoàn thiện — plan T3

- [ ] T012 [P] Cập nhật README.md: thêm hàng `/` (`Filter active pane by name (Esc clears)`) vào bảng "In SFTP File Manager" và một gạch đầu dòng ở mục "SFTP File Manager"
- [ ] T013 Commit `docs: document sftp file filter` (README.md, specs/005-sftp-filter/tasks.md)

---

## Phụ thuộc & thứ tự

- Phase 3 → Phase 4 (T007–T009 gọi `SetFilter`, `step`, đọc `filter`) → Phase 5.
- T007–T009 cùng sửa `sftp_view.go` nên tuần tự, chung một commit (plan T2).
- T012 độc lập về file, có thể làm song song với Phase 4.

## Ví dụ chạy song song

```text
Phase 4 (sftp_view.go) ‖ T012 (README.md)
```

## Chiến lược hiện thực

- **MVP**: Phase 3 + T007–T008 (US1 + US2): lọc và thao tác trên kết quả.
- Tăng dần: T009 (US3, Esc) rồi tài liệu. Mỗi phase để `go test -race ./...` xanh.
