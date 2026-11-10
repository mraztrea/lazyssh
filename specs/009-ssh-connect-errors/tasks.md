---

description: "Danh sách task cho Báo lỗi kết nối SSH/SFTP và xử lý host key đã đổi"
---

# Tasks: Báo lỗi kết nối SSH/SFTP và xử lý host key đã đổi

**Input**: Tài liệu thiết kế trong `/specs/009-ssh-connect-errors/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Bắt buộc. Hiến pháp II: viết test, chạy và thấy nó FAIL, hiện thực, thấy PASS, commit.
Test chỉ dùng `t.TempDir()`; không đụng `~/.ssh`, `~/.lazyssh`. Kết nối thật kiểm theo
quickstart.md với `sshd` tạm và config/known_hosts tạm.

**Organization**: Mỗi phase kết thúc bằng một commit (message kết thúc bằng dòng `Co-Authored-By`
của phiên). Ánh xạ plan: Phase 2 = plan T2; Phase 3 = plan T3 (trừ `RemoveKnownHost`) + T4;
Phase 4 = plan T5 (hộp thoại lỗi thường); Phase 5 = `RemoveKnownHost` (plan T3) + hộp thoại host key
(plan T5); Phase 6 = plan T6.

## Format: `[ID] [P?] [Story] Mô tả`

- **[P]**: Có thể chạy song song (file khác nhau, không phụ thuộc)
- **[Story]**: US1 host key đổi, US2 lỗi kết nối khác, US3 thoát phiên bình thường

## Phase 1: Setup

- [X] T001 Commit tài liệu `docs: add spec, plan and tasks for ssh connect errors` (specs/009-ssh-connect-errors/*)

---

## Phase 2: Nền tảng (Foundational) — plan T2

**Mục tiêu**: hàm thuần `classifySSHError` biến stderr của OpenSSH thành `*domain.SSHError`.

**Kiểm thử độc lập**: `go test -race ./internal/core/services/ -run ClassifySSHError -v`

- [X] T002 Tạo internal/core/domain/ssh_error.go (license header): `SSHError{Reason, Hint, Output string; HostKey *HostKeyChange}` với `Error() string { return e.Reason }`; `HostKeyChange{Host, File string; Line int; KeyType, Fingerprint string}`. Tạo internal/core/services/ssh_error_test.go với `TestClassifySSHError` dạng bảng dùng nguyên văn thông báo trong research.md R3: banner host key đổi của người dùng (cổng 22: `Host key for 103.142.24.161 has changed`, `Offending ECDSA key in /home/u/.ssh/known_hosts:19`, ED25519 `SHA256:k3nqAxo38Pg0bgF4cy2RBqL7Pn9EY0P1U3K0st0ABls`) → Reason `Host key changed`, HostKey đủ 5 trường; banner cổng 22422 với `\r\n` → Host `[127.0.0.1]:22422`, Line 1; banner thiếu dòng `Offending` → Reason `Host key changed`, `HostKey == nil`; `No ED25519 host key is known for … Host key verification failed.` → `Host key not verified`; UNPROTECTED PRIVATE KEY + `Permission denied` → `Private key permissions too open`; `Permission denied (publickey,…)` → `Authentication failed`; `Too many authentication failures` → `Authentication failed`; `Connection refused` → `Connection refused`; `Connection timed out` → `Connection timed out`; `Could not resolve hostname` → `Host name not found`; `No route to host` và `Network is unreachable` → `Host unreachable`; `Bad configuration option: foo` → `SSH connection failed` với Output chứa nguyên văn dòng đó; stderr rỗng → Output bằng fallback; 12 dòng → Output đúng 8 dòng cuối, không có `\r`. Mọi Reason trừ trường hợp không nhận diện có Hint khác rỗng. Tạo internal/core/services/ssh_error.go chỉ có header + `package services`; chạy test, xác nhận FAIL (undefined)
- [X] T003 Hiện thực trong internal/core/services/ssh_error.go: `classifySSHError(stderr, fallback string) *domain.SSHError` (bỏ `\r`; bảng `sshFailures` theo thứ tự R3 so khớp `strings.Contains`; Output = tối đa 8 dòng cuối không rỗng, hoặc fallback; nhánh host key đổi dùng `hostKeyChange(text)` với regex `(?i)host key for (\S+) has changed`, `(?m)^Offending (\S+) key in (.+):(\d+)$`, `fingerprint for the (\S+) key sent by the remote host is\s+(\S+)` bỏ `.` cuối; trả nil nếu thiếu Host hoặc File) và `storedPasswordError(err error) *domain.SSHError` (Reason `Stored password unavailable`). Chạy test, `gofmt -l .`, `go vet ./...`; xác nhận PASS
- [X] T004 Commit `feat: classify ssh connection errors from openssh output` (ssh_error.go ×2, ssh_error_test.go, tasks.md)

---

## Phase 3: User Story 3 + 2 (service) — thoát bình thường im lặng, lỗi được trả về (Priority: P1)

**Mục tiêu**: `SSH`/`SSHWithArgs` trả `nil` cho phiên kết thúc bình thường/bị ngắt, `*domain.SSHError`
cho exit 255; terminal vẫn thấy stderr; SFTP trả lỗi đã phân loại.

**Kiểm thử độc lập**: `go test -race ./internal/core/services/ -run 'RunSSH|Connect_' -v`; quickstart bước 3.

- [X] T005 [US3] Trong internal/core/services/server_service_test.go thêm `TestRunSSH_ExitStatus`: thư mục tạm chứa script `ssh` (`#!/bin/sh` in `$FAKE_SSH_ERR` ra stderr, `exit $FAKE_SSH_CODE`), `t.Setenv("PATH", dir)`; repo giả `fakeRepo{ports.ServerRepository; recorded []string; pwErr error}` hiện thực `GetPassword`, `RecordSSH`. Bảng: 255 + `ssh: connect to host h port 22: Connection refused` → `errors.As` ra `*domain.SSHError` Reason `Connection refused`, không ghi; 3 → nil, không ghi; 0 → nil, ghi `a`; `pwErr` → Reason `Stored password unavailable`. Áp dụng cả `SSH` và `SSHWithArgs`. Trong internal/core/services/sftp_service_test.go sửa `TestConnect_ProcessExitReturnsError` đòi `errors.As(err, &se)` và `se.Reason == "Connection refused"`. Chạy, xác nhận FAIL
- [X] T006 [US3] Trong internal/core/services/server_service.go thay thân `SSH`/`SSHWithArgs` bằng `runSSH(server, extraArgs)` theo plan T3 (`io.MultiWriter(os.Stderr, tail)`, `WaitDelay = time.Second`, bỏ qua `exec.ErrWaitDelay`, 255 → `classifySSHError(tail.String(), "ssh exited with status 255")`, ExitError khác → log + nil, `sshCmd` lỗi → `storedPasswordError`). Trong internal/core/services/sftp_service.go: `passwordAuth` lỗi → `storedPasswordError(err)`; bắt tay lỗi → `classifySSHError(stderr.String(), err.Error())`; xoá `tailBuffer.lastLine`. Chạy `go test -race ./...`, `gofmt -l .`, `go vet ./...`; xác nhận PASS
- [X] T007 [US3] Commit `feat: return classified errors when ssh or sftp fails to connect`

---

## Phase 4: User Story 2 — hộp thoại lỗi kết nối (Priority: P1)

**Mục tiêu**: Enter, Forward + SSH và `m` hiện hộp thoại `Retry` / `Edit server` / `Close`
(mặc định Close) khi kết nối lỗi.

**Kiểm thử độc lập**: `go test -race ./internal/adapters/ui/ -run ConnectError -v`; quickstart bước 4–6, 8.

- [X] T008 [US2] Tạo internal/adapters/ui/connect_error_test.go với `TestConnectErrorText`: `connectErrorText("web", &domain.SSHError{Reason: "Connection refused", Hint: "h", Output: "ssh: connect to host [::1] port 22: Connection refused"})` chứa `web: Connection refused`, `h` và Output đã escape (`[::1[]`… theo `tview.Escape`). Tạo internal/adapters/ui/connect_error.go chỉ có header; chạy, xác nhận FAIL
- [X] T009 [US2] Hiện thực trong internal/adapters/ui/connect_error.go: `connectErrorText`, `showConnectError(server domain.Server, err error, retry func())` (lỗi không phải `*domain.SSHError` → `Reason = err.Error()`; modal tiêu đề ` Connection failed `, nút `Retry`/`Edit server`/`Close`, `SetFocus(2)`; mọi nút gọi `returnToMain` trước rồi `retry()` hoặc `showServerEditForm(server)`). Trong internal/adapters/ui/handlers.go: tách `showServerEditForm(server)` khỏi `handleServerEdit`; `handleServerConnect` → `connectSSH(server)` (Suspend SSH, `refreshServerList`, lỗi → `showConnectError` với retry chính nó); Forward + SSH → `connectSSHWithArgs(server, args)` tương tự. Trong internal/adapters/ui/sftp_view.go: `handleFileManager` → `openFileManager(server)`; lỗi → `showConnectError(server, err, func() { t.openFileManager(server) })`. Chạy `go build ./...`, `go test -race ./...`, `gofmt -l .`, `go vet ./...`; xác nhận PASS
- [X] T010 [US2] Commit `feat: show a dialog when ssh or sftp fails to connect`

---

## Phase 5: User Story 1 — host key đổi: cảnh báo, xoá key cũ, kết nối lại (Priority: P1)

**Mục tiêu**: hộp thoại cảnh báo MITM với `Cancel` (mặc định) / `Remove old key & reconnect`;
xoá bằng `ssh-keygen -R host -f file` chỉ khi người dùng chọn.

**Kiểm thử độc lập**: `go test -race ./internal/core/services/ -run RemoveKnownHost -v`,
`go test -race ./internal/adapters/ui/ -run HostKey -v`; quickstart bước 1–3, 7.

- [ ] T011 [US1] Trong internal/core/services/server_service_test.go thêm `TestRemoveKnownHost` (skip nếu `exec.LookPath("ssh-keygen")` lỗi): sinh key ed25519 bằng `ssh-keygen -q -t ed25519 -N "" -f <tmp>/k`, ghi `<tmp>/known_hosts` hai dòng `[127.0.0.1]:2222 <pub>` và `other <pub>`; `RemoveKnownHost("[127.0.0.1]:2222", file)` → nil, file chỉ còn `other`, có `known_hosts.old`; host hoặc file rỗng → lỗi; file không tồn tại → lỗi. Trong internal/adapters/ui/connect_error_test.go thêm `TestHostKeyText`: với `HostKeyChange{Host: "[127.0.0.1]:2222", File: "/k/known_hosts", Line: 19, KeyType: "ED25519", Fingerprint: "SHA256:abc"}` văn bản chứa `man-in-the-middle`, `[127.0.0.1[]:2222` (đã escape), `ED25519`, `SHA256:abc`, `/k/known_hosts:19`. Chạy, xác nhận FAIL
- [ ] T012 [US1] Thêm `RemoveKnownHost(host, file string) error` vào `ServerService` trong internal/core/ports/services.go và hiện thực trong internal/core/services/server_service.go (`exec.Command("ssh-keygen", "-R", host, "-f", file).CombinedOutput()`; lỗi kèm đầu ra; log host+file). Trong internal/adapters/ui/connect_error.go: `hostKeyText(alias, hk)` và nhánh `HostKey != nil` của `showConnectError`: modal tiêu đề ` Host key changed `, nút `Cancel`/`Remove old key & reconnect`, focus 0; chỉ `buttonIndex == 1` mới gọi `RemoveKnownHost`; lỗi → `showConnectError` với Reason `Could not remove the old host key`; ok → `retry()`. Chạy `go build ./...`, `go test -race ./...`, `gofmt -l .`, `go vet ./...`; xác nhận PASS
- [ ] T013 [US1] Commit `feat: offer to remove a changed host key and reconnect`

---

## Phase 6: Hoàn thiện — plan T6

- [ ] T014 Chạy quickstart.md phần end-to-end với `sshd` tạm (ghi kết quả vào báo cáo; phần không tự động được thì ghi rõ)
- [ ] T015 Cập nhật README.md (Quick Server Navigation: hộp thoại lỗi kết nối, luồng host key đổi, `.old`) và commit `docs: document connection error dialogs`

## Phụ thuộc

Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6. US3 và phần service của US2 cùng một thay đổi
(`runSSH`); US1 cần hộp thoại của US2. T008 [P] có thể viết song song với T005 (file khác).

## Chiến lược

MVP = Phase 2–4 (mọi lỗi đều được báo, kể cả host key đổi dưới dạng lỗi thường có cảnh báo); Phase
5 thêm hành động xoá key cũ và kết nối lại.
