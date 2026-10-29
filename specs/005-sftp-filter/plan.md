# Kế hoạch hiện thực: Lọc file trong trình quản lý file SFTP

**Branch**: `005-sftp-filter` | **Ngày**: 2026-10-29 | **Spec**: [spec.md](./spec.md)

**Input**: Đặc tả tính năng từ `/specs/005-sftp-filter/spec.md`

## Tóm tắt

`filePane` giữ danh sách đầy đủ (`all`) và chuỗi lọc (`filter`); `Load` chỉ list + sort rồi gọi
`render()`, còn `render()` dựng bảng từ `entries = filterEntries(all, filter)` nên `Selected()` vẫn
đúng chỉ số. `fileManager` có thêm một `tview.InputField` thay chỗ dòng hints trong layout khi người
dùng bấm `/`; `SetChangedFunc` gọi `active.SetFilter`, `SetInputCapture` bắt Lên/Xuống/Enter/Tab/Esc
để trả focus về bảng. `handleKey` của layout bỏ qua mọi phím khi ô filter đang focus để phím chữ
thành text.

## Bối cảnh kỹ thuật (Technical Context)

**Language/Version**: Go 1.26 (`go 1.26.0` trong go.mod)

**Primary Dependencies**: `tview`/`tcell` có sẵn. Không thêm module mới.

**Storage**: không có (filter chỉ nằm trong bộ nhớ, không lưu)

**Testing**: `go test -race ./...`; unit test thuần Go trong `internal/adapters/ui/sftp_pane_test.go`
với `list` giả (kiểm `GetCell`, `GetRowCount`, `GetTitle`, `Selected`). Phần nối phím tcell kiểm thủ
công theo [quickstart.md](./quickstart.md).

**Target Platform**: Linux, macOS, Windows (terminal mà tcell hỗ trợ)

**Project Type**: công cụ CLI TUI trên desktop

**Performance Goals**: lọc O(n) trên danh sách đã nạp mỗi lần gõ; 1.000 entry không có độ trễ nhận
thấy được (SC-002)

**Constraints**: không đổi hành vi phím tắt khi không dùng filter (SC-004); không truy vấn lại server
khi gõ

**Scale/Scope**: 3 task; 0 file mới; sửa `sftp_pane.go`, `sftp_view.go`, `sftp_pane_test.go`,
`README.md`

## Kiểm tra hiến pháp (Constitution Check)

*GATE: Phải qua trước Phase 0 research. Kiểm lại sau Phase 1 design.*

| Nguyên tắc | Kiểm tra | Trạng thái |
|---|---|---|
| I. Phân lớp hexagonal | Chỉ sửa UI adapter `internal/adapters/ui`; không import mới. | PASS |
| II. Test trước | T1 bắt đầu bằng test thất bại cho `filterEntries`, `SetFilter` (hàng, `../`, title), `Load` xoá/giữ filter, `step`. T2 là phần nối phím tcell, kiểm thủ công (quickstart) vì dựa trên `tview.Application` thật; logic nó gọi đã được T1 test. | PASS |
| III. Ít dependency | Chỉ `tview`/`tcell`/stdlib `strings`. | PASS |
| IV. An toàn bí mật | Không chạm bí mật, không ghi file. | PASS |
| V. Đơn giản (YAGNI) | Không interface/option mới; một hàm thuần, hai method trên `filePane`, ba method trên `fileManager`; tái dùng pattern `search_bar.go`. Không regex/glob. | PASS |

Kiểm lại sau thiết kế: PASS; Complexity Tracking trống.

## Cấu trúc dự án (Project Structure)

### Tài liệu (tính năng này)

```text
specs/005-sftp-filter/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── checklists/requirements.md
└── tasks.md
```

Không có `contracts/`: tính năng chỉ là hành vi phím trong TUI, đã mô tả trong spec và quickstart.

### Mã nguồn (gốc repository)

```text
internal/adapters/ui/
├── sftp_pane.go          # filterEntries; filePane.all/filter; Load → render(); SetFilter; step (T1)
├── sftp_pane_test.go     # test cho các hàm trên (T1)
├── sftp_view.go          # filterInput, startFilter/endFilter, handleKey '/', hints (T2)
README.md                 # bảng phím SFTP + gạch đầu dòng tính năng (T3)
```

**Structure Decision**: giữ bố cục hiện có; không file mới.

## Thiết kế theo task

### T1: Lọc trong `filePane`

- `filterEntries(e []fileEntry, q string) []fileEntry`: `q == ""` → trả `e`; ngược lại giữ entry có
  `strings.Contains(strings.ToLower(Name), strings.ToLower(q))`, giữ thứ tự.
- `filePane` thêm `all []fileEntry` (đầy đủ, đã sort) và `filter string`; `entries` là danh sách đã
  lọc (giữ tên để `Selected()` không đổi).
- `Load(dir)`: list lỗi → pane không đổi (như cũ); `dir != p.cwd` → `p.filter = ""` (FR-007);
  `p.all, p.cwd = entries, dir`; `p.render()`.
- `render()`: phần dựng bảng cũ của `Load`, lặp trên `p.entries = filterEntries(p.all, p.filter)`;
  title `" %s: %s%s "` + nếu có filter thì `tview.Escape(" [/"+filter+"]")` trước khoảng trắng cuối;
  `Select(1, 0)`, `ScrollToBeginning()`.
- `SetFilter(q)`: `p.filter = q; p.render()`.
- `step(d int)`: chọn `min(max(row+d, 1), GetRowCount()-1)` (không lên header, không vượt cuối).
- `reload` trong `sftp_view.go` gọi `Load(p.cwd)` cùng thư mục nên giữ filter, rồi kẹp hàng như cũ.
- Test: `TestFilterEntries` (rỗng, hoa thường, substring, không khớp), `TestFilePane_SetFilter`
  (số hàng, `../` còn, `Selected()` đúng entry đã lọc, title có `[/...]`, chuỗi rỗng khôi phục),
  `TestFilePane_LoadClearsFilterOnDirChange` (cùng dir giữ, dir khác xoá),
  `TestFilePane_Step` (kẹp ở `../` và hàng cuối; không khớp → ở `../`).

### T2: Ô filter trong `fileManager`

- `fileManager.filterInput *tview.InputField` (label `Filter: `, nền `Color235` như `status`), tạo
  trong `showFileManager`.
- `SetChangedFunc(func(q) { fm.active.SetFilter(q) })`.
- `SetInputCapture`: `KeyDown`, `KeyEnter` → `endFilter(1)`; `KeyUp` → `endFilter(-1)`; `KeyTab` →
  `endFilter(0)`; `KeyEscape` → `SetText("")` (kích hoạt changed → hiện đầy đủ) rồi `endFilter(0)`;
  phím khác → trả về cho InputField.
- `startFilter()`: `SetText(fm.active.filter)`, `layout.RemoveItem(status)`,
  `layout.AddItem(filterInput, 1, 0, false)`, `app.SetFocus(filterInput)`.
- `endFilter(d)`: `RemoveItem(filterInput)`, `AddItem(status, 1, 0, false)`,
  `app.SetFocus(fm.active)`, `fm.active.step(d)` khi `d != 0`.
- `handleKey`: dòng đầu `if fm.filterInput.HasFocus() { return ev }` (layout capture chạy trước
  InputField nên phải nhường phím); map rune thêm `'/': fm.startFilter`.
- `fileManagerHints` thêm `[white]/[-] Filter`.
- Kiểm thủ công theo quickstart (Constitution II: phần nối tcell không có harness test sẵn trong
  repo; logic được T1 phủ).

### T3: Tài liệu

- README: thêm hàng `/` vào bảng phím SFTP và một gạch đầu dòng ở mục SFTP File Manager.

## Complexity Tracking

Trống.
