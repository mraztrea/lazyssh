# Kế hoạch hiện thực: Xem/sửa file bằng editor trong trình quản lý file SFTP

**Branch**: `007-sftp-edit` | **Ngày**: 2026-10-29 | **Spec**: [spec.md](./spec.md)

**Input**: Đặc tả tính năng từ `/specs/007-sftp-edit/spec.md`

## Tóm tắt

File mới `internal/core/services/sftp_edit.go` có hai hàm thuần: `EditorCommand(env string)
[]string` (tách `$EDITOR` bằng `strings.Fields`, mặc định `["vi"]`) và `EditRemote(c, remotePath,
run) (changed bool, err error)` (tải file remote về thư mục tạm riêng, gọi `run`, so SHA-256 trước/
sau, ghi đè remote bằng `remoteSide(c).create` nếu đổi, xoá thư mục tạm trừ khi tải lên lỗi). Hai
helper nội bộ `copyFile` (tái dùng `side` của `sftp_transfer.go`) và `hashFile`. Trong UI,
`fileManager` thêm phím `e` → `edit()`: Local chạy editor trong `app.Suspend`; Remote giữ cờ `busy`
rồi gọi `services.EditRemote` trong `app.Suspend` với `run = runEditor`. Kết quả hiện ở dòng status.

## Bối cảnh kỹ thuật (Technical Context)

**Language/Version**: Go 1.26 (`go 1.26.0` trong go.mod)

**Primary Dependencies**: `github.com/pkg/sftp`, `tview`/`tcell` có sẵn; stdlib `crypto/sha256`,
`os/exec`. Không thêm module mới.

**Storage**: bản sao tạm trong `os.MkdirTemp("", "lazyssh-edit-")` (thư mục 0700, file 0600); không
ghi gì dưới `~/.lazyssh`.

**Testing**: `go test -race ./...`; unit test `internal/core/services/sftp_edit_test.go` dùng
`pipeClient` (sftp server in-process trong `sftp_transfer_test.go`) và `t.Setenv("TMPDIR",
t.TempDir())` để thư mục tạm nằm trong `t.TempDir()`. Phần nối phím tcell và chạy editor thật kiểm
thủ công theo [quickstart.md](./quickstart.md).

**Target Platform**: Linux, macOS (terminal mà tcell hỗ trợ; `vi` có sẵn). Windows: cần đặt
`EDITOR`.

**Project Type**: công cụ CLI TUI trên desktop

**Performance Goals**: không có mục tiêu riêng; file lớn chỉ tốn thời gian tải xuống/tải lên.

**Constraints**: không đổi hành vi phím tắt hiện có (SC-005); bản sao tạm 0600 (FR-004); không mất
bản sửa khi tải lên lỗi (FR-007).

**Scale/Scope**: 4 task; 2 file mới (`sftp_edit.go`, `sftp_edit_test.go`); sửa `sftp_view.go`,
`README.md`.

## Kiểm tra hiến pháp (Constitution Check)

*GATE: Phải qua trước Phase 0 research. Kiểm lại sau Phase 1 design.*

| Nguyên tắc | Kiểm tra | Trạng thái |
|---|---|---|
| I. Phân lớp hexagonal | Logic sửa remote nằm ở `services`; UI chỉ nối phím, `Suspend`, `exec` và hiển thị. UI gọi hàm package-level của `services` giống `UploadPath`/`DownloadPath`/`RemoveRemote` đã có trong `sftp_view.go`; không import `adapters/data`. | PASS |
| II. Test trước | T1: test `EditorCommand` thất bại trước. T2: test `EditRemote` thất bại trước cho đổi/không đổi/`run` lỗi/tải xuống lỗi/tải lên lỗi (giữ bản sao), quyền 0600 của bản sao, giữ mode remote, xoá thư mục tạm. T3 là phần nối phím và `exec` editor với terminal thật, kiểm thủ công (quickstart); logic nó gọi đã được T1, T2 test. | PASS |
| III. Ít dependency | Chỉ stdlib và `pkg/sftp` có sẵn. | PASS |
| IV. An toàn bí mật | Không ghi dưới `~/.lazyssh`. Bản sao tạm của file remote (có thể chứa bí mật) nằm trong thư mục 0700, file 0600, xoá sau khi xong. | PASS |
| V. Đơn giản (YAGNI) | Không interface/option/cấu hình mới; tái dùng `side`, `localSide`, `remoteSide`, cờ `busy`, `reload`/`after`. Không thêm cấu hình editor trong lazyssh. | PASS |

Kiểm lại sau thiết kế: PASS; Complexity Tracking trống.

## Cấu trúc dự án (Project Structure)

### Tài liệu (tính năng này)

```text
specs/007-sftp-edit/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── checklists/requirements.md
└── tasks.md
```

Không có `contracts/`: tính năng là hành vi phím trong TUI (đã mô tả trong spec và quickstart) và
hai hàm nội bộ (mô tả trong [data-model.md](./data-model.md)).

### Mã nguồn (gốc repository)

```text
internal/core/services/
├── sftp_edit.go          # MỚI: EditorCommand (T1); EditRemote, copyFile, hashFile (T2)
├── sftp_edit_test.go     # MỚI: test cho các phần trên (T1, T2)
internal/adapters/ui/
├── sftp_view.go          # handleKey 'e', edit, runEditor, hints (T3)
README.md                 # bảng phím SFTP + gạch đầu dòng tính năng (T4)
```

**Structure Decision**: giữ bố cục hiện có; logic mới cạnh `sftp_transfer.go` để tái dùng `side`.

## Thiết kế theo task

### T1: Chọn editor

- `EditorCommand(env string) []string`: `strings.Fields(env)`; rỗng → `[]string{"vi"}` (FR-011).
- Test bảng: `""` → `[vi]`, `"   "` → `[vi]`, `"nano"` → `[nano]`, `"code --wait"` →
  `[code --wait]`.

### T2: Luồng sửa remote

- `EditRemote(c *sftp.Client, remotePath string, run func(localPath string) error) (bool, error)`:
  1. `dir := os.MkdirTemp("", "lazyssh-edit-")` (0700); `defer` xoá `dir` trừ khi `keep`.
  2. `local := filepath.Join(dir, path.Base(remotePath))` (giữ tên/đuôi để editor tô màu cú pháp).
  3. `copyFile(remoteSide(c), localSide(), remotePath, local)` — `localSide().create` tạo file 0600
     (FR-004); không chmod theo mode remote.
  4. `before := hashFile(local)`; `run(local)` — lỗi thì trả lỗi, không tải lên (FR-008).
  5. `after := hashFile(local)`; bằng nhau → `(false, nil)` (FR-006).
  6. `copyFile(localSide(), remoteSide(c), local, remotePath)` — `O_WRONLY|O_CREATE|O_TRUNC`, không
     chmod nên giữ mode remote (FR-005). Lỗi → `keep = true`, trả `(true, fmt.Errorf("%w (edited
     copy kept at %s)", err, local))` (FR-007).
- `copyFile(from, to side, src, dst string) error`: open, create, `io.Copy`, đóng dst và trả lỗi
  close (lỗi ghi SFTP thường lộ ra khi close).
- `hashFile(p string) ([sha256.Size]byte, error)`.
- Test (đều `t.Setenv("TMPDIR", t.TempDir())`, remote là file thật qua `pipeClient`):
  - `run` ghi nội dung mới → `changed`, remote mang nội dung mới, mode remote 0640 giữ nguyên, bản
    sao trong `run` có mode 0600 và tên gốc, thư mục tạm đã bị xoá.
  - `run` không ghi (hoặc ghi lại đúng nội dung cũ) → `!changed`, remote giữ nguyên, thư mục tạm bị
    xoá.
  - `run` trả lỗi sau khi sửa bản sao → lỗi đó được trả, remote giữ nguyên, thư mục tạm bị xoá.
  - remote không tồn tại → lỗi, `run` không được gọi, `TMPDIR` rỗng.
  - tải lên lỗi (trong `run`, thay file remote bằng một thư mục cùng tên → mở ghi thất bại kể cả khi
    chạy bằng root) → lỗi chứa đường dẫn bản sao, bản sao còn với nội dung đã sửa.

### T3: Phím trong `fileManager`

- `runEditor(file string) error`: `exec.Command` với `EditorCommand(os.Getenv("EDITOR"))` + `file`,
  `Stdin/Stdout/Stderr = os.Stdin/os.Stdout/os.Stderr`, `Run()`.
- `edit()`: `p := fm.active`; `fm.selected(p)` + bỏ qua `IsDir` (FR-001).
  - Local: `fm.t.app.Suspend(func() { err = runEditor(target) })`, rồi `fm.edited(p, err,
    "Edited "+name)` (FR-003).
  - Remote: `busy.CompareAndSwap(false, true)` thất bại → `fail("A transfer is already running")`
    (FR-012); `Suspend` gọi `services.EditRemote(fm.client, target, runEditor)`; `busy.Store(false)`;
    `fm.edited(p, err, "Saved "+name | "No changes")` (FR-005, FR-006).
- `edited(p, err, msg)`: lỗi → `reload(p)` (bỏ qua lỗi reload) rồi `fail("Edit failed: " + err)`
  (FR-008, FR-009); ngược lại `fm.after(p, nil, msg)`.
- `handleKey` thêm `'e': fm.edit`; ô filter có focus thì phím là text (đã có sẵn).
- `fileManagerHints` thêm `[white]e[-] Edit` (FR-010).

### T4: Tài liệu

- README: hàng `e` trong bảng "In SFTP File Manager" và một gạch đầu dòng ở mục tính năng SFTP.

## Complexity Tracking

Trống.
