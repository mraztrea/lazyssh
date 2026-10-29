# Kế hoạch hiện thực: Thư mục làm việc mặc định (Home Dir) cho mỗi profile SSH

**Branch**: `008-profile-home-dir` | **Ngày**: 2026-10-29 | **Spec**: [spec.md](./spec.md)

**Input**: Đặc tả tính năng từ `/specs/008-profile-home-dir/spec.md`

## Tóm tắt

Thêm trường `HomeDir` vào `domain.Server`, lưu trong `~/.lazyssh/metadata.json` (khoá `home_dir`)
cạnh `tags`, nên tự đi theo sync và theo đổi alias. File mới
`internal/core/services/home_dir.go` có hai hàm thuần: `HomeDirCommand(s domain.Server) string`
(sinh lệnh remote `cd <dir>; exec "$SHELL" -l` hoặc `cd <dir>; <RemoteCommand>`, quoting an toàn,
`~` → `"$HOME"`) và `ResolveRemoteDir(dir, home string) string` (đổi `~`/đường dẫn tương đối thành
đường dẫn tuyệt đối cho SFTP). `ServerService.SSH`/`SSHWithArgs` nhận `domain.Server` thay vì alias
và, khi có Home Dir, chạy `ssh -o RemoteCommand=none -t … <alias> '<lệnh>'`. Form Add/Edit thêm ô
`Home Dir:`, pane chi tiết thêm dòng `Home Dir`, file manager SFTP nạp pane Remote tại thư mục đã
resolve (lỗi thì về thư mục mặc định và báo đỏ).

## Bối cảnh kỹ thuật (Technical Context)

**Language/Version**: Go 1.26 (`go 1.26.0` trong go.mod)

**Primary Dependencies**: stdlib (`strings`, `path`, `os/exec`), `pkg/sftp`, `tview`/`tcell` có sẵn.
Không thêm module mới.

**Storage**: `~/.lazyssh/metadata.json`, khoá mới `home_dir` (chuỗi, `omitempty`) trong
`ServerMetadata`. Không đụng `~/.ssh/config`.

**Testing**: `go test -race ./...`. Unit test cho `HomeDirCommand`, `ResolveRemoteDir`, `sshArgs`,
`validateServer` (xuống dòng), round-trip/rename metadata (`t.TempDir()`), và render pane chi tiết.
Kết nối ssh/sftp thật kiểm thủ công theo [quickstart.md](./quickstart.md).

**Target Platform**: Linux, macOS; tài khoản remote có shell POSIX/fish (`cd`, `;`, `exec`,
`"$SHELL"`, `"$HOME"`).

**Project Type**: công cụ CLI TUI trên desktop

**Performance Goals**: không có mục tiêu riêng; thêm tối đa 3 tham số vào lệnh ssh.

**Constraints**: Home Dir rỗng → lệnh ssh và thư mục đầu SFTP y hệt hiện tại (FR-006, SC-003); không
thực thi nội dung Home Dir như lệnh (FR-010); `cd` thất bại vẫn vào shell (FR-005).

**Scale/Scope**: 5 task; 2 file mới (`home_dir.go`, `home_dir_test.go`); sửa `domain/server.go`,
`ports/services.go`, `metadata_manager.go`, `mapper.go`, `server_service.go` (+test), `server_form.go`,
`server_details.go` (+test), `field_help.go`, `defaults.go`, `handlers.go`, `sftp_view.go`,
`README.md`.

## Kiểm tra hiến pháp (Constitution Check)

*GATE: Phải qua trước Phase 0 research. Kiểm lại sau Phase 1 design.*

| Nguyên tắc | Kiểm tra | Trạng thái |
|---|---|---|
| I. Phân lớp hexagonal | `domain` chỉ thêm một trường. Sinh lệnh remote và resolve đường dẫn nằm ở `services`; UI gọi `services.ResolveRemoteDir` như đã gọi `services.EditorCommand`. Lưu trữ nằm trong adapter `ssh_config_file`. Port `ServerService.SSH`/`SSHWithArgs` đổi tham số sang `domain.Server` (UI đã có sẵn server được chọn). Không import chéo. | PASS |
| II. Test trước | T1: test metadata round-trip/rename và `validateServer` thất bại trước. T2: test `HomeDirCommand` (rỗng, tuyệt đối, `~`, `~/x`, khoảng trắng, dấu `'`, kết hợp RemoteCommand, `none`), `ResolveRemoteDir`, `sshArgs` thất bại trước. T3: test pane chi tiết hiện/ẩn `Home Dir`. T4/T5: nối UI và tài liệu, kiểm thủ công; logic được gọi đã có test. | PASS |
| III. Ít dependency | Chỉ stdlib. | PASS |
| IV. An toàn bí mật | Home Dir không phải bí mật; không log `domain.Server`; metadata vẫn ghi 0600 qua `saveAll` hiện có. | PASS |
| V. Đơn giản (YAGNI) | Không interface/option mới; một trường, hai hàm thuần, một tham số `remote` thêm vào `sshCmd`. Không kiểm tra tồn tại thư mục lúc lưu, không bookmark, không nhớ thư mục cuối. | PASS |

Kiểm lại sau thiết kế: PASS; Complexity Tracking trống.

## Cấu trúc dự án (Project Structure)

### Tài liệu (tính năng này)

```text
specs/008-profile-home-dir/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── checklists/requirements.md
└── tasks.md
```

Không có `contracts/`: giao diện bên ngoài duy nhất là khoá `home_dir` trong metadata và dạng lệnh ssh,
cả hai mô tả trong [data-model.md](./data-model.md).

### Mã nguồn (gốc repository)

```text
internal/core/domain/server.go                     # HomeDir string (T1)
internal/adapters/data/ssh_config_file/
├── metadata_manager.go                            # ServerMetadata.HomeDir; updateServer merge (T1)
├── metadata_manager_test.go                       # round-trip + rename (T1)
├── mapper.go                                      # mergeMetadata copies HomeDir (T1)
internal/core/services/
├── server_service.go                              # validateServer; SSH/SSHWithArgs(domain.Server); sshCmd remote (T1, T2)
├── server_service_test.go                         # validateServer newline; sshArgs (T1, T2)
├── home_dir.go                                    # MỚI: HomeDirCommand, ResolveRemoteDir, shellQuote (T2)
├── home_dir_test.go                               # MỚI (T2)
internal/core/ports/services.go                    # SSH(server), SSHWithArgs(server, args) (T2)
internal/adapters/ui/
├── handlers.go                                    # truyền server thay vì alias (T2)
├── server_form.go                                 # ô Home Dir: (T3)
├── field_help.go, defaults.go                     # help + placeholder HomeDir (T3)
├── server_details.go (+_test.go)                  # dòng Home Dir (T3)
├── sftp_view.go                                   # pane Remote nạp Home Dir (T4)
README.md                                          # tính năng (T5)
```

**Structure Decision**: giữ bố cục hiện có; logic thuần cạnh `sftp_edit.go` theo cùng kiểu hàm
package-level.

## Thiết kế theo task

### T1: Lưu trữ Home Dir

- `domain.Server.HomeDir string` (comment: remote working dir; lives in metadata, never in ssh config).
- `ServerMetadata.HomeDir string \`json:"home_dir,omitempty"\``; `mergeMetadata`:
  `servers[i].HomeDir = meta.HomeDir`; `updateServer`: `merged.HomeDir = server.HomeDir` (ngay sau
  `merged.Tags`). Rename alias đã được xử lý bởi khối `oldAlias != server.Alias`.
- `validateServer`: `strings.ContainsAny(srv.HomeDir, "\r\n")` → `fmt.Errorf("home dir must be a
  single line")` (FR-001).
- Test: `TestMetadata_HomeDirRoundTripAndRename` (lưu → load có `home_dir`; sửa Tags giữ HomeDir; đổi
  alias mang theo; xoá trắng → khoá biến mất); `validateServer` với `"a\nb"` lỗi, `"/var/www"` OK.

### T2: Lệnh ssh và resolve đường dẫn

- `home_dir.go`:
  - `shellQuote(s string) string`: `'` + `strings.ReplaceAll(s, "'", "'\\''")` + `'`.
  - `homeDirExpr(dir string) string`: `~` → `"$HOME"`; `~/rest` → `"$HOME"/` + `shellQuote(rest)`;
    khác → `shellQuote(dir)`.
  - `HomeDirCommand(s domain.Server) string`: `dir := strings.TrimSpace(s.HomeDir)`; rỗng → `""`.
    `rest := exec "$SHELL" -l`; nếu `s.RemoteCommand != ""` và `!= "none"` → `rest = s.RemoteCommand`.
    Trả `cd <expr>; <rest>` (dấu `;` để `cd` lỗi vẫn vào shell, FR-005).
  - `ResolveRemoteDir(dir, home string) string`: trim; rỗng → `home`; `~` → `home`; `~/x` →
    `path.Join(home, x)`; không bắt đầu bằng `/` → `path.Join(home, dir)`; còn lại `path.Clean(dir)`.
- `server_service.go`:
  - `sshArgs(alias string, authArgs, pre, post []string, remote string) []string` (thuần): nếu
    `remote != ""` thì `pre` được thêm trước `-o RemoteCommand=none -t`; thứ tự
    `pre, auth, post, alias, [remote]`.
  - `sshCmd(alias, pre, post, remote)` dùng `sshArgs`; `StartForward` truyền `""`.
  - `SSH(server domain.Server)` và `SSHWithArgs(server domain.Server, extraArgs)` truyền
    `HomeDirCommand(server)`; log vẫn chỉ `alias`.
- `ports/services.go`: đổi chữ ký hai hàm. `handlers.go`: `t.serverService.SSH(server)`;
  form port-forward truyền `server` (đã có trong scope của `showPortForwardForm`).
- Test `home_dir_test.go` (bảng): HomeDir rỗng/khoảng trắng → `""`; `/var/www/app` →
  `cd '/var/www/app'; exec "$SHELL" -l`; `~` → `cd "$HOME"; …`; `~/a b` → `cd "$HOME"/'a b'; …`;
  `it's` → `'it'\''s'`; RemoteCommand `tmux a` → `cd '/x'; tmux a`; RemoteCommand `none` → exec shell.
  `ResolveRemoteDir`: `""`→home, `~`→home, `~/a`→home/a, `a/b`→home/a/b, `/a/../b`→`/b`.
  `sshArgs`: không remote → `[pre auth post alias]`; có remote → `[-o RemoteCommand=none -t pre auth
  post alias remote]`.

### T3: Form và pane chi tiết

- `ServerFormData.HomeDir`; Edit: `HomeDir: sf.original.HomeDir`; Add: `""`; thu thập
  `getFieldText("Home Dir:")`; dựng server `HomeDir: strings.TrimSpace(data.HomeDir)`.
- Ô nhập ngay sau `Tags:` ở tab cơ bản: `sf.addInputFieldWithHelp(form, "Home Dir:", "HomeDir",
  defaultValues.HomeDir, 40, GetFieldPlaceholder("HomeDir"))`.
- `fieldHelpData["HomeDir"]`: mô tả "Remote directory to start in after SSH/SFTP connect (lazyssh
  only, not written to ssh config)", Examples `/var/www/app`, `~/projects`, Default `none`, Category
  `lazyssh`. `GetFieldPlaceholder("HomeDir")` → `e.g. /var/www/app`.
- `server_details.go`: thêm `{"Home Dir", server.HomeDir}` vào một group mới `lazyssh` trong `groups`
  (tên group không được in; trường chỉ xuất hiện trong phần Advanced khi khác rỗng, như các trường
  khác). Test: render với HomeDir có `Home Dir: /x`; không có thì
  không chứa `Home Dir`.

### T4: SFTP

- `fileManager.homeDir string`; `handleFileManager` → `showFileManager(server, sess)` gán
  `fm.homeDir = server.HomeDir` (alias lấy từ `server.Alias`).
- Khởi tạo pane Remote:
  ```go
  wd, err := fm.client.Getwd(); if err != nil { wd = "/" }
  start := services.ResolveRemoteDir(fm.homeDir, wd)
  err = fm.remote.Load(start)
  if err != nil && start != wd { fm.fail("Home dir not found: " + fm.homeDir); err = fm.remote.Load(wd) }
  if err != nil { fm.fail(err.Error()) }
  ```
  `ResolveRemoteDir("", wd) == wd` nên không có Home Dir thì hành vi y cũ (FR-007, FR-008).

### T5: Tài liệu

- README mục Server Management: gạch đầu dòng `📁 Home Dir:` … (SSH mở shell tại đó, SFTP mở pane
  Remote tại đó, lưu trong metadata và được sync).

## Complexity Tracking

Trống.
