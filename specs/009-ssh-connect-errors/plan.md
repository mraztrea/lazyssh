# Kế hoạch hiện thực: Báo lỗi kết nối SSH/SFTP và xử lý host key đã đổi

**Branch**: `009-ssh-connect-errors` | **Ngày**: 2026-11-10 | **Spec**: [spec.md](./spec.md)

**Input**: Đặc tả tính năng từ `/specs/009-ssh-connect-errors/spec.md`

## Tóm tắt

Nguyên nhân gốc: `handleServerConnect` gọi `_ = t.serverService.SSH(server)` trong `app.Suspend`,
lỗi bị bỏ và TUI vẽ đè lên stderr của ssh. Sửa: `ServerService.SSH`/`SSHWithArgs` nhân bản stderr
của ssh vào `tailBuffer` sẵn có (terminal vẫn thấy như cũ); khi ssh thoát 255, trả
`*domain.SSHError` do hàm thuần `classifySSHError` dựng từ phần đuôi stderr; mã khác (phiên kết
thúc, Ctrl-C) trả `nil`. `SFTPService.Connect` dùng cùng hàm phân loại. UI có một file mới
`connect_error.go` hiện `tview.Modal`: lỗi thường với `Retry` / `Edit server` / `Close`, host key
đổi với `Cancel` / `Remove old key & reconnect`; hành động xoá gọi port mới
`ServerService.RemoveKnownHost(host, file)` (`ssh-keygen -R host -f file`) rồi chạy lại đúng thao
tác ban đầu để ssh tự hỏi xác nhận fingerprint mới.

## Bối cảnh kỹ thuật (Technical Context)

**Language/Version**: Go 1.26 (`go 1.26.0` trong go.mod; máy dev Go 1.27.3)

**Primary Dependencies**: stdlib (`io`, `os/exec`, `regexp`, `errors`), `tview`/`tcell` có sẵn,
`ssh` và `ssh-keygen` của OpenSSH trên máy người dùng. Không thêm module.

**Storage**: không có. Chỉ sửa `known_hosts` khi người dùng chọn xoá (qua `ssh-keygen`, có `.old`).

**Testing**: `go test -race ./...`. Bảng test `classifySSHError` dùng nguyên văn thông báo OpenSSH
10.6 thu được khi chạy thật (research R3); test luồng mã thoát với `ssh` giả trong `PATH`
(`t.TempDir()`); test `RemoveKnownHost` với `known_hosts` tạm (bỏ qua nếu thiếu `ssh-keygen`); test
văn bản hộp thoại (escape). Kết nối thật kiểm theo [quickstart.md](./quickstart.md).

**Target Platform**: Linux, macOS với OpenSSH.

**Project Type**: công cụ CLI TUI trên desktop

**Performance Goals**: không đáng kể; bộ đệm stderr tối đa 4 KiB mỗi phiên.

**Constraints**: không đổi argv của ssh; không nới kiểm tra host key (FR-008); không đụng
`known_hosts` khi chưa có xác nhận (FR-007); phiên bình thường không hộp thoại (FR-002).

**Scale/Scope**: 6 task. File mới: `domain/ssh_error.go`, `services/ssh_error.go` (+test),
`ui/connect_error.go` (+test). Sửa: `ports/services.go`, `services/server_service.go` (+test),
`services/sftp_service.go` (+test), `ui/handlers.go`, `ui/sftp_view.go`, `README.md`.

## Kiểm tra hiến pháp (Constitution Check)

*GATE: Phải qua trước Phase 0 research. Kiểm lại sau Phase 1 design.*

| Nguyên tắc | Kiểm tra | Trạng thái |
|---|---|---|
| I. Phân lớp hexagonal | `SSHError`/`HostKeyChange` ở `domain` (không import gì). Phân loại và chạy `ssh-keygen` ở `services`; UI chỉ gọi port `ServerService.RemoveKnownHost` và `errors.As` trên kiểu domain. Không import `adapters/data`. | PASS |
| II. Test trước | T2: bảng test phân loại thất bại trước khi có hàm. T3: test `runSSH` (ssh giả: 255/3/0, thiếu mật khẩu) và `RemoveKnownHost` thất bại trước. T4: test SFTP đòi `*domain.SSHError`. T5: test văn bản hộp thoại (escape `[host]:port`, nội dung bắt buộc). Test chỉ dùng `t.TempDir()`, không đụng `~/.ssh`. | PASS |
| III. Ít dependency | Chỉ stdlib + công cụ OpenSSH đã là điều kiện chạy lazyssh. | PASS |
| IV. An toàn bí mật | `Output` chỉ là stderr của ssh (ssh không in mật khẩu) hoặc thông báo lỗi đọc metadata (không chứa bí mật). Log vẫn chỉ theo alias. | PASS |
| V. Đơn giản (YAGNI) | Không interface mới, không tuỳ chọn cấu hình; tái dùng `tailBuffer`, `tview.Modal`; một bộ nút cho mọi lỗi thường; gợi ý nằm trong một bảng. | PASS |

Kiểm lại sau thiết kế: PASS; Complexity Tracking trống.

## Cấu trúc dự án (Project Structure)

### Tài liệu (tính năng này)

```text
specs/009-ssh-connect-errors/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── checklists/requirements.md
└── tasks.md
```

Không có `contracts/`: giao diện ra ngoài duy nhất là lệnh `ssh-keygen -R` và hộp thoại, mô tả trong
[data-model.md](./data-model.md).

### Mã nguồn (gốc repository)

```text
internal/core/domain/ssh_error.go                  # MỚI: SSHError, HostKeyChange (T2)
internal/core/services/
├── ssh_error.go                                   # MỚI: classifySSHError, storedPasswordError (T2)
├── ssh_error_test.go                              # MỚI (T2)
├── server_service.go                              # runSSH (tee, mã thoát), RemoveKnownHost (T3)
├── server_service_test.go                         # (T3)
├── sftp_service.go (+_test.go)                    # Connect trả lỗi đã phân loại (T4)
internal/core/ports/services.go                    # ServerService.RemoveKnownHost (T3)
internal/adapters/ui/
├── connect_error.go                               # MỚI: showConnectError, hộp thoại host key (T5)
├── connect_error_test.go                          # MỚI (T5)
├── handlers.go                                    # connect/forward dùng hộp thoại; showServerEditForm (T5)
├── sftp_view.go                                   # handleFileManager dùng hộp thoại (T5)
README.md                                          # (T6)
```

**Structure Decision**: giữ bố cục hiện có; hàm phân loại là hàm package-level như `home_dir.go`.

## Thiết kế theo task

### T2: Kiểu lỗi và bộ phân loại

- `domain.SSHError{Reason, Hint, Output string; HostKey *HostKeyChange}`, `Error() = Reason`;
  `domain.HostKeyChange{Host, File string; Line int; KeyType, Fingerprint string}`.
- `classifySSHError(stderr, fallback string) *domain.SSHError`: bỏ `\r`; bảng `[]struct{match
  []string; reason, hint string}` duyệt theo thứ tự R3 (so `strings.Contains`); `Output` = 8 dòng
  cuối không rỗng hoặc `fallback`. Nhánh host key đổi: regex
  `(?i)host key for (\S+) has changed`, `(?m)^Offending (\S+) key in (.+):(\d+)$`,
  `fingerprint for the (\S+) key sent by the remote host is\s+(\S+)`; chỉ gán `HostKey` khi có cả
  Host và File.
- `storedPasswordError(err)`: Reason `Stored password unavailable`, Hint về Edit server /
  `~/.lazyssh/master.key`, Output `err.Error()`.
- Test bảng: mỗi dòng R3 với nguyên văn thông báo (kể cả banner đầy đủ của người dùng: cổng 22,
  `ECDSA` cũ / `ED25519` mới, dòng 19), cổng khác 22, thiếu dòng Offending → `HostKey == nil`,
  `\r\n`, > 8 dòng, rỗng → fallback, không nhận diện → Reason chung + nguyên văn.

### T3: ServerService

- `SSH`/`SSHWithArgs` gọi chung `runSSH(server, extraArgs)`: `sshCmd` lỗi → `storedPasswordError`;
  `cmd.Stderr = io.MultiWriter(os.Stderr, tail)`, `cmd.WaitDelay = time.Second`;
  `errors.Is(err, exec.ErrWaitDelay)` → coi như `nil`; `ExitError` 255 →
  `classifySSHError(tail.String(), "ssh exited with status 255")`; `ExitError` khác → log, `nil`, không
  `RecordSSH` (giữ như cũ); lỗi khác → trả nguyên; thành công → `RecordSSH`.
- `RemoveKnownHost(host, file)`: rỗng → lỗi; `exec.Command("ssh-keygen", "-R", host, "-f", file)`
  `.CombinedOutput()`; lỗi → `fmt.Errorf("ssh-keygen -R %s: %w: %s", …)`.
- Port thêm `RemoveKnownHost(host, file string) error`.
- Test: `PATH` trỏ tới thư mục tạm có script `ssh` giả (in stderr, thoát theo biến môi trường); repo
  giả nhúng `ports.ServerRepository` chỉ hiện thực `GetPassword`/`RecordSSH`. 255 → `SSHError`
  `Connection refused`, không ghi; 3 → `nil`, không ghi; 0 → `nil`, ghi; `GetPassword` lỗi →
  `Stored password unavailable`. `RemoveKnownHost`: file tạm hai host, xoá `[127.0.0.1]:2222`, host
  kia còn, có `.old`.

### T4: SFTP

- `Connect`: `passwordAuth` lỗi → `storedPasswordError(err)`; bắt tay lỗi →
  `classifySSHError(stderr.String(), err.Error())`. `tailBuffer.lastLine` hết dùng → xoá.
- Test `TestConnect_ProcessExitReturnsError` đòi `errors.As(*domain.SSHError)` với Reason
  `Connection refused`.

### T5: Giao diện

- `connect_error.go`:
  - `connectErrorText(alias, se)`: `alias: Reason`, dòng trống, Hint, dòng trống, Output; đi qua
    `tview.Escape`.
  - `hostKeyText(alias, hk)`: cảnh báo MITM, `Host:`, `New <type> key: <fp>`, `Old key: file:line`,
    giải thích ssh sẽ hỏi xác nhận; `tview.Escape`.
  - `showConnectError(server, err, retry func())`: không phải `SSHError` → `Reason = err.Error()`.
    `HostKey != nil` → modal `Cancel`/`Remove old key & reconnect` (focus 0; Esc = -1 = huỷ); chọn
    xoá → `RemoveKnownHost` → lỗi: `showConnectError` với Reason `Could not remove the old host key`;
    ok: `retry()`. Còn lại → modal `Retry`/`Edit server`/`Close`, `SetFocus(2)`.
- `handlers.go`: `handleServerConnect` → `t.connect(server)`; `connect` chạy `Suspend(SSH)`,
  `refreshServerList`, lỗi → `showConnectError(server, err, func(){ t.connect(server) })`. Forward +
  SSH tương tự với `SSHWithArgs(server, args)`. Tách `showServerEditForm(server)` khỏi
  `handleServerEdit`.
- `sftp_view.go`: `handleFileManager` → `openFileManager(server)`; lỗi →
  `showConnectError(server, err, func(){ t.openFileManager(server) })`.
- Test: `hostKeyText` chứa `\[127.0.0.1]:2222` (đã escape), fingerprint, `file:19`,
  `man-in-the-middle`; `connectErrorText` chứa Reason/Hint/Output.

### T6: Tài liệu

- README: gạch đầu dòng ở Quick Server Navigation về hộp thoại lỗi kết nối và luồng host key đổi.

## Complexity Tracking

Trống.
