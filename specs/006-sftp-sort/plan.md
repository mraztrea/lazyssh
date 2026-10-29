# Kế hoạch hiện thực: Sắp xếp file trong trình quản lý file SFTP

**Branch**: `006-sftp-sort` | **Ngày**: 2026-10-29 | **Spec**: [spec.md](./spec.md)

**Input**: Đặc tả tính năng từ `/specs/006-sftp-sort/spec.md`

## Tóm tắt

`filePane` có thêm trường `sort paneSort` (hai bool: `byTime`, `desc`). `sortEntries(e, s)` mở rộng
hàm hiện có: thư mục trước, rồi theo tên hoặc mtime theo chiều; cùng mtime thì theo tên tăng dần.
Việc sort chuyển từ `Load` sang `render()` (trước khi lọc) nên mọi đường nạp lại, lọc và đổi chế độ
đều dùng chế độ hiện tại. `SetSort` render lại và giữ hàng con trỏ. `handleKey` của `fileManager`
thêm `s`/`S`, báo `Sort: <mode>` trên dòng status. Cột Modified đổi layout sang
`2006-01-02 15:04:05`.

## Bối cảnh kỹ thuật (Technical Context)

**Language/Version**: Go 1.26 (`go 1.26.0` trong go.mod)

**Primary Dependencies**: `tview`/`tcell` có sẵn. Không thêm module mới.

**Storage**: không có (chế độ sort chỉ nằm trong bộ nhớ, không lưu)

**Testing**: `go test -race ./...`; unit test thuần Go trong `internal/adapters/ui/sftp_pane_test.go`
với `list` giả (kiểm thứ tự, `GetCell`, `GetTitle`, `GetSelection`). Phần nối phím tcell kiểm thủ
công theo [quickstart.md](./quickstart.md).

**Target Platform**: Linux, macOS, Windows (terminal mà tcell hỗ trợ)

**Project Type**: công cụ CLI TUI trên desktop

**Performance Goals**: sort O(n log n) trên danh sách đã nạp mỗi lần render; 1.000 entry không có độ
trễ nhận thấy được (SC-003)

**Constraints**: không đổi hành vi phím tắt hiện có (SC-004); không truy vấn lại server khi đổi chế
độ

**Scale/Scope**: 4 task; 0 file mới; sửa `sftp_pane.go`, `sftp_pane_test.go`, `sftp_view.go`,
`README.md`

## Kiểm tra hiến pháp (Constitution Check)

*GATE: Phải qua trước Phase 0 research. Kiểm lại sau Phase 1 design.*

| Nguyên tắc | Kiểm tra | Trạng thái |
|---|---|---|
| I. Phân lớp hexagonal | Chỉ sửa UI adapter `internal/adapters/ui`; không import mới. | PASS |
| II. Test trước | T1 bắt đầu bằng test thất bại cho ô Modified; T2 bằng test thất bại cho `sortEntries` bốn chế độ, tie-break, `SetSort` (thứ tự, title, giữ con trỏ, giữ filter), `Load` giữ chế độ. T3 là phần nối phím tcell, kiểm thủ công (quickstart) vì dựa trên `tview.Application` thật; logic nó gọi đã được T2 test. | PASS |
| III. Ít dependency | Chỉ `tview`/`tcell`/stdlib `sort`, `strings`, `time`. | PASS |
| IV. An toàn bí mật | Không chạm bí mật, không ghi file. | PASS |
| V. Đơn giản (YAGNI) | Không interface/option mới; một struct hai bool với `String()`, một method `SetSort`, một method `sortBy` trên `fileManager`. Không tái dùng `SortMode` của màn hình chính (nhãn `Alias`/`Last SSH` và quy tắc pinned không khớp). | PASS |

Kiểm lại sau thiết kế: PASS; Complexity Tracking trống.

## Cấu trúc dự án (Project Structure)

### Tài liệu (tính năng này)

```text
specs/006-sftp-sort/
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
├── sftp_pane.go          # layout cột Modified (T1); paneSort, sortEntries(e, s), filePane.sort, render sort, SetSort (T2)
├── sftp_pane_test.go     # test cho các phần trên (T1, T2)
├── sftp_view.go          # handleKey 's'/'S', sortBy, hints (T3)
README.md                 # bảng phím SFTP + gạch đầu dòng tính năng (T4)
```

**Structure Decision**: giữ bố cục hiện có; không file mới.

## Thiết kế theo task

### T1: Thời gian sửa đầy đủ

- `render()`: ô cột 2 là `" " + e.ModTime.Format("2006-01-02 15:04:05")` (giờ local, FR-008).
- Test `TestFilePane_ModifiedColumn`: entry có `ModTime = time.Date(2026, 10, 29, 14, 5, 9, 0,
  time.Local)` → `GetCell(2, 2).Text == " 2026-10-29 14:05:09"`.

### T2: Sắp xếp trong `filePane`

- `type paneSort struct{ byTime, desc bool }`; giá trị rỗng là `Name ↑` (FR-001).
  `String()` → `Name ↑`, `Name ↓`, `Modified ↑`, `Modified ↓`.
- `sortEntries(e []fileEntry, s paneSort)`: `sort.SliceStable`; thư mục trước; `byTime` và mtime
  khác nhau → so mtime theo chiều; `byTime` (mtime bằng nhau) → tên thường tăng dần; theo tên →
  tên thường theo chiều (FR-004).
- `filePane.sort paneSort`; `Load` bỏ lời gọi `sortEntries`; `render()` gọi
  `sortEntries(p.all, p.sort)` trước `filterEntries` (FR-007).
- Title: `" %s: %s%s"` + `tview.Escape(" [" + p.sort.String() + "]")` + phần filter cũ + `" "`
  (FR-005).
- `SetSort(s paneSort)`: nhớ hàng đang chọn, gán, `render()`, chọn lại hàng đó (số hàng không đổi
  nên không cần kẹp) (FR-006).
- Test: sửa `TestSortEntries_DirsFirstCaseInsensitive` sang `sortEntries(e, paneSort{})`; thêm
  `TestSortEntries_Modes` (bốn chế độ, thư mục trước, tie-break cùng mtime theo tên tăng dần ở cả
  hai chiều), `TestPaneSort_String`, `TestFilePane_SetSort` (thứ tự hàng, title có `[Modified ↓`,
  con trỏ giữ hàng, filter giữ và vẫn lọc, `Load` thư mục khác giữ chế độ).

### T3: Phím trong `fileManager`

- `sortBy(s paneSort)`: `fm.active.SetSort(s)`, `fm.info("Sort: " + s.String())` (FR-006).
- `handleKey` map rune thêm `'s'` → `sortBy(paneSort{!cur.byTime, cur.desc})` và `'S'` →
  `sortBy(paneSort{cur.byTime, !cur.desc})` với `cur = fm.active.sort` (FR-002, FR-003). Khi ô
  filter có focus `handleKey` đã trả phím nguyên vẹn nên `s`/`S` là text.
- `fileManagerHints` thêm `[white]s[-] Sort  • [white]S[-] Reverse` (FR-009).
- Kiểm thủ công theo quickstart.

### T4: Tài liệu

- README: thêm hàng `s`, `S` vào bảng "In SFTP File Manager" và một gạch đầu dòng ở mục SFTP File
  Manager.

## Complexity Tracking

Trống.
