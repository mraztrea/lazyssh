---

description: "Danh sách task cho Xem/sửa file bằng editor trong trình quản lý file SFTP"
---

# Tasks: Xem/sửa file bằng editor trong trình quản lý file SFTP

**Input**: Tài liệu thiết kế trong `/specs/007-sftp-edit/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Bắt buộc. Hiến pháp II: viết test, chạy và thấy nó FAIL, hiện thực, thấy PASS, commit.
Test service dùng `pipeClient` (sftp server in-process trong `sftp_transfer_test.go`) và
`t.Setenv("TMPDIR", t.TempDir())`. Phần nối phím tcell và chạy editor thật (T009–T010) được kiểm thủ
công theo quickstart.md.

**Organization**: Mỗi phase ứng với một task trong plan.md ("Thiết kế theo task", T1–T4); mỗi phase
kết thúc bằng một commit với message kết thúc bằng dòng `Co-Authored-By` của phiên. Chọn editor
(plan T1) phục vụ mọi story nên là nền tảng; luồng sửa remote (plan T2) phục vụ US1 và US3; phần nối
phím (plan T3) hoàn tất US1, US2, US3.

## Format: `[ID] [P?] [Story] Mô tả`

- **[P]**: Có thể chạy song song (file khác nhau, không phụ thuộc)
- **[Story]**: US1 sửa file remote, US2 sửa file local, US3 báo lỗi

## Phase 1: Setup

Không có task setup: module, công cụ và test helper đã có; không thêm dependency.

---

## Phase 2: Nền tảng (Foundational) — plan T1

**Mục tiêu**: Chọn lệnh editor từ `EDITOR`, mặc định `vi`.

**Kiểm thử độc lập**: `go test -race ./internal/core/services/ -run EditorCommand -v`

- [X] T001 Tạo internal/core/services/sftp_edit_test.go (license header Apache) với `TestEditorCommand` dạng bảng: `""` → `[vi]`, `"   "` → `[vi]`, `"nano"` → `[nano]`, `"code --wait"` → `[code --wait]`; tạo internal/core/services/sftp_edit.go (license header) chỉ với `package services` để test biên dịch được tới lỗi "undefined: EditorCommand"; chạy và xác nhận FAIL
- [X] T002 Trong internal/core/services/sftp_edit.go thêm `EditorCommand(env string) []string`: `strings.Fields(env)`, rỗng → `[]string{"vi"}`; chạy `go test -race ./...`, `gofmt -l .`, `go vet ./...`; xác nhận PASS
- [X] T003 Commit `feat: pick sftp editor from $EDITOR` (sftp_edit.go, sftp_edit_test.go, tasks.md)

---

## Phase 3: User Story 1 + 3 — Luồng sửa remote (Priority: P1/P2) — plan T2

**Mục tiêu**: `EditRemote` tải file về bản sao tạm 0600, gọi editor, ghi đè remote khi nội dung đổi
(giữ mode), báo không đổi, trả lỗi rõ ràng, dọn thư mục tạm trừ khi tải lên lỗi.

**Kiểm thử độc lập**: `go test -race ./internal/core/services/ -run EditRemote -v`

- [X] T004 [US1] Trong internal/core/services/sftp_edit_test.go thêm (mỗi test `t.Setenv("TMPDIR", t.TempDir())`, `c := pipeClient(t)`, remote là file trong `t.TempDir()` tạo bằng `writeFile(..., 0o640)`): `TestEditRemote_Changed` — `run` kiểm `filepath.Base(local)` bằng tên remote và mode bản sao là 0600, ghi nội dung mới → `changed == true`, `err == nil`, `assertFile` remote mang nội dung mới, mode remote vẫn 0640, `os.Stat(filepath.Dir(local))` trả `fs.ErrNotExist`; `TestEditRemote_Unchanged` — `run` ghi lại đúng nội dung cũ → `changed == false`, remote giữ nguyên, thư mục tạm đã bị xoá
- [X] T005 [US3] Cùng file thêm: `TestEditRemote_RunError` — `run` ghi nội dung mới rồi trả `errors.New("boom")` → `errors.Is(err, boom)`, `changed == false`, remote giữ nguyên, thư mục tạm bị xoá; `TestEditRemote_DownloadError` — remote không tồn tại → lỗi, `run` không được gọi, `TMPDIR` rỗng; `TestEditRemote_UploadErrorKeepsCopy` — `run` ghi nội dung mới vào bản sao rồi thay file remote bằng một thư mục cùng tên (`os.Remove` + `os.Mkdir`) → lỗi chứa đường dẫn bản sao, bản sao còn với nội dung đã sửa
- [X] T006 Chạy `go test ./internal/core/services/ -run EditRemote -v`; xác nhận FAIL (chưa có `EditRemote`)
- [X] T007 [US1] Trong internal/core/services/sftp_edit.go thêm `EditRemote(c *sftp.Client, remotePath string, run func(localPath string) error) (changed bool, err error)` theo vòng đời ở data-model.md: `os.MkdirTemp("", "lazyssh-edit-")`, `defer` xoá trừ khi `keep`; bản sao `filepath.Join(dir, path.Base(remotePath))` tải bằng `copyFile(remoteSide(c), localSide(), ...)` ("file mode 0600"); `hashFile` trước/sau `run`; bằng nhau → `(false, nil)`; khác → `copyFile(localSide(), remoteSide(c), local, remotePath)` (không chmod, giữ mode remote); lỗi tải lên → `keep = true`, `fmt.Errorf("%w (edited copy kept at %s)", err, local)`; helper `copyFile(from, to side, src, dst string) error` (open, create, `io.Copy`, trả lỗi close của dst) và `hashFile(p string) ([sha256.Size]byte, error)`
- [X] T008 Chạy `go test -race ./...`, `gofmt -l .`, `go vet ./...`; xác nhận PASS; commit `feat: edit remote sftp files through a private temp copy` (sftp_edit.go, sftp_edit_test.go, tasks.md)

---

## Phase 4: User Story 1 + 2 + 3 — Phím `e` trong file manager (Priority: P1/P2) — plan T3

**Mục tiêu**: `e` mở editor cho file đang chọn: Local sửa trực tiếp, Remote qua `EditRemote`; kết
quả `Edited <name>` / `Saved <name>` / `No changes` / `Edit failed: <lỗi>`; tôn trọng cờ `busy`.

**Kiểm thử độc lập**: quickstart.md mục "Thủ công" bước 1–11.

- [X] T009 [US1] Trong internal/adapters/ui/sftp_view.go: `runEditor(file string) error` (`exec.Command` với `services.EditorCommand(os.Getenv("EDITOR"))` + `file`, stdio kế thừa `os.Stdin/os.Stdout/os.Stderr`, `Run()`); `edit()` (bỏ qua khi `!ok` hoặc `e.IsDir`; Local: `fm.t.app.Suspend` chạy `runEditor` rồi `fm.edited(p, err, "Edited "+name)`; Remote: `busy.CompareAndSwap(false, true)` thất bại → `fm.fail("A transfer is already running")`, ngược lại `Suspend` gọi `services.EditRemote(fm.client, target, runEditor)`, `busy.Store(false)`, `fm.edited(p, err, "Saved "+name | "No changes")`); `edited(p, err, msg)` (lỗi → `reload(p)` bỏ qua lỗi reload rồi `fm.fail("Edit failed: " + err.Error())`, ngược lại `fm.after(p, nil, msg)`); map rune của `handleKey` thêm `'e': fm.edit`; `fileManagerHints` thêm `[white]e[-] Edit`
- [X] T010 Chạy `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test -race ./...`; xác nhận PASS; kiểm các bước thủ công trong quickstart.md nếu có terminal
- [X] T011 Commit `feat: add e key to edit files in sftp file manager` (sftp_view.go, tasks.md)

---

## Phase 5: Hoàn thiện — plan T4

- [X] T012 [P] Cập nhật README.md: thêm hàng `e` (`Edit selected file in $EDITOR (vi)`) vào bảng "In SFTP File Manager" và một gạch đầu dòng ở mục tính năng SFTP
- [X] T013 Commit `docs: document sftp file edit` (README.md, specs/007-sftp-edit/tasks.md)

---

## Phụ thuộc & thứ tự

- Phase 2 → Phase 3 (cùng file) → Phase 4 (T009 gọi `EditorCommand`, `EditRemote`) → Phase 5.
- T012 độc lập về file, có thể làm song song với Phase 4.

## Ví dụ chạy song song

```text
Phase 4 (sftp_view.go) ‖ T012 (README.md)
```

## Chiến lược hiện thực

- **MVP**: Phase 2 + Phase 3 + Phase 4 (US1): sửa file remote bằng một phím.
- US2 (Local) và US3 (báo lỗi) đi cùng Phase 4 vì chung hàm `edit()`/`edited()`; rồi tài liệu. Mỗi
  phase để `go test -race ./...` xanh.
