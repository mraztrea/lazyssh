---

description: "Danh sách task cho Thư mục làm việc mặc định (Home Dir) cho mỗi profile SSH"
---

# Tasks: Thư mục làm việc mặc định (Home Dir) cho mỗi profile SSH

**Input**: Tài liệu thiết kế trong `/specs/008-profile-home-dir/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Bắt buộc. Hiến pháp II: viết test, chạy và thấy nó FAIL, hiện thực, thấy PASS, commit.
Test metadata dùng `newSecretRepo(t)` / `rawMetadata(t, meta)` có sẵn trong
`metadata_manager_test.go` (`t.TempDir()`). Kết nối ssh/sftp thật kiểm thủ công theo quickstart.md.

**Organization**: Mỗi phase ứng với một task trong plan.md ("Thiết kế theo task", T1–T5); mỗi phase
kết thúc bằng một commit với message kết thúc bằng dòng `Co-Authored-By` của phiên. Lưu trữ (plan T1)
là nền tảng cho mọi story; lệnh ssh (plan T2) hoàn tất US1; SFTP (plan T4) hoàn tất US2; form và
pane chi tiết (plan T3) hoàn tất US3.

## Format: `[ID] [P?] [Story] Mô tả`

- **[P]**: Có thể chạy song song (file khác nhau, không phụ thuộc)
- **[Story]**: US1 SSH vào Home Dir, US2 SFTP mở tại Home Dir, US3 cấu hình/xem trong giao diện

## Phase 1: Setup

Không có task setup: module, công cụ và test helper đã có; không thêm dependency.

---

## Phase 2: Nền tảng (Foundational) — plan T1

**Mục tiêu**: `domain.Server.HomeDir` được lưu/đọc qua `~/.lazyssh/metadata.json` (khoá `home_dir`,
`omitempty`), đi theo đổi alias, sống sót khi sửa trường khác; từ chối giá trị nhiều dòng.

**Kiểm thử độc lập**: `go test -race ./internal/adapters/data/ssh_config_file/ -run HomeDir -v` và
`go test -race ./internal/core/services/ -run ValidateServer_HomeDir -v`

- [X] T001 Trong internal/adapters/data/ssh_config_file/metadata_manager_test.go thêm `TestRepo_HomeDirRoundTripAndRename`: `repo, _, meta := newSecretRepo(t)`; `AddServer(domain.Server{Alias: "a", Host: "h", HomeDir: "/var/www"})` → `rawMetadata(t, meta)["a"]["home_dir"] == "/var/www"`; `ListServers("")` trả server `a` có `HomeDir == "/var/www"`; `UpdateServer` chỉ đổi `Tags` → HomeDir giữ nguyên; `UpdateServer` đổi alias sang `b` → `rawMetadata` có `b.home_dir == "/var/www"` và không còn `a`; `UpdateServer` với `HomeDir: ""` → khoá `home_dir` biến mất khỏi `rawMetadata(...)["b"]`. Trong internal/core/services/server_service_test.go thêm `TestValidateServer_HomeDirSingleLine`: `HomeDir: "a\nb"` → lỗi `"home dir must be a single line"`; `HomeDir: "/var/www"` → nil. Chạy hai test; xác nhận FAIL (biên dịch lỗi "unknown field HomeDir")
- [X] T002 Thêm trường `HomeDir string` vào `domain.Server` trong internal/core/domain/server.go (comment: remote working dir for ssh/sftp; stored in metadata.json, never in ssh config); `HomeDir string \`json:"home_dir,omitempty"\`` vào `ServerMetadata` trong internal/adapters/data/ssh_config_file/metadata_manager.go và `merged.HomeDir = server.HomeDir` ngay sau `merged.Tags = server.Tags` trong `updateServer`; `servers[i].HomeDir = meta.HomeDir` trong `mergeMetadata` ở internal/adapters/data/ssh_config_file/mapper.go; trong `validateServer` ở internal/core/services/server_service.go thêm `if strings.ContainsAny(srv.HomeDir, "\r\n") { return fmt.Errorf("home dir must be a single line") }`. Chạy `go test -race ./...`, `gofmt -l .`, `go vet ./...`; xác nhận PASS
- [X] T003 Commit `feat: store per-server home dir in metadata` (server.go, metadata_manager.go, metadata_manager_test.go, mapper.go, server_service.go, server_service_test.go, tasks.md)

---

## Phase 3: User Story 1 — SSH vào thẳng Home Dir (Priority: P1) — plan T2

**Mục tiêu**: Phiên SSH tương tác (Enter, port forward tương tác) chạy
`ssh -o RemoteCommand=none -t … <alias> 'cd <dir>; exec "$SHELL" -l'` (hoặc `cd <dir>; <RemoteCommand>`);
Home Dir rỗng → argv y hệt hiện tại; port forward nền không thêm lệnh remote.

**Kiểm thử độc lập**: `go test -race ./internal/core/services/ -run 'HomeDirCommand|ResolveRemoteDir|SSHArgs' -v`; quickstart.md bước 2, 4, 5, 6, 8.

- [X] T004 [US1] Tạo internal/core/services/home_dir_test.go (license header Apache 2.0 như các file khác) với `TestHomeDirCommand` dạng bảng trên `domain.Server`: `HomeDir: ""` → `""`; `"   "` → `""`; `"/var/www/app"` → `cd '/var/www/app'; exec "$SHELL" -l`; `"~"` → `cd "$HOME"; exec "$SHELL" -l`; `"~/a b"` → `cd "$HOME"/'a b'; exec "$SHELL" -l`; `"/it's"` → `cd '/it'\''s'; exec "$SHELL" -l`; `"/x; rm -rf /"` → `cd '/x; rm -rf /'; exec "$SHELL" -l`; `HomeDir: "/x", RemoteCommand: "tmux attach || tmux new"` → `cd '/x'; tmux attach || tmux new`; `HomeDir: "/x", RemoteCommand: "none"` → `cd '/x'; exec "$SHELL" -l`. `TestResolveRemoteDir` dạng bảng với home `/home/u`: `""` → `/home/u`; `"  "` → `/home/u`; `"~"` → `/home/u`; `"~/a"` → `/home/u/a`; `"a/b"` → `/home/u/a/b`; `"/a/../b"` → `/b`; `"/var/www/"` → `/var/www`. Trong internal/core/services/server_service_test.go thêm `TestSSHArgs`: `sshArgs("h", []string{"-o","A"}, []string{"-L","1:2:3"}, []string{"-N"}, "")` → `[-L 1:2:3 -o A -N h]`; với remote `cd '/x'; exec "$SHELL" -l` → `[-o RemoteCommand=none -t -L 1:2:3 -o A -N h cd '/x'; exec "$SHELL" -l]` (lệnh là một phần tử). Tạo internal/core/services/home_dir.go chỉ với license header và `package services`; chạy test, xác nhận FAIL (undefined)
- [X] T005 [US1] Trong internal/core/services/home_dir.go hiện thực `shellQuote(s string) string` (`'` + `strings.ReplaceAll(s, "'", "'\\''")` + `'`), `homeDirExpr(dir string) string` (`~` → `"$HOME"`; tiền tố `~/` → `"$HOME"/` + `shellQuote(rest)`; khác → `shellQuote(dir)`), `HomeDirCommand(s domain.Server) string` (trim; rỗng → `""`; `rest := exec "$SHELL" -l`, nếu `s.RemoteCommand != ""` và `!= "none"` thì `rest = s.RemoteCommand`; trả `"cd " + expr + "; " + rest`; comment một dòng: `;` để cd lỗi vẫn vào shell), `ResolveRemoteDir(dir, home string) string` (trim; rỗng hoặc `~` → `home`; `~/x` → `path.Join(home, x)`; không bắt đầu bằng `/` → `path.Join(home, dir)`; còn lại `path.Clean(dir)`). Trong internal/core/services/server_service.go thêm `sshArgs(alias string, authArgs, pre, post []string, remote string) []string` (nếu `remote != ""` thì bắt đầu bằng `-o RemoteCommand=none -t`; rồi `pre, authArgs, post, alias`; rồi `remote` nếu khác rỗng) và đổi `sshCmd(alias string, pre, post []string, remote string)` dùng nó; `StartForward` truyền `""`. Chạy test; xác nhận PASS
- [X] T006 [US1] Đổi port trong internal/core/ports/services.go: `SSH(server domain.Server) error`, `SSHWithArgs(server domain.Server, extraArgs []string) error`. Trong internal/core/services/server_service.go đổi `SSH`/`SSHWithArgs` nhận `server domain.Server`, dùng `alias := server.Alias` cho log/`RecordSSH` và truyền `HomeDirCommand(server)` vào `sshCmd`. Trong internal/adapters/ui/handlers.go: `handleServerConnect` gọi `t.serverService.SSH(server)`; trong `showPortForwardForm(server domain.Server)` gọi `t.serverService.SSHWithArgs(server, args)`. Chạy `go build ./...`, `go test -race ./...`, `gofmt -l .`, `go vet ./...`; xác nhận PASS
- [X] T007 [US1] Commit `feat: start ssh sessions in the server home dir` (home_dir.go, home_dir_test.go, server_service.go, server_service_test.go, services.go, handlers.go, tasks.md)

---

## Phase 4: User Story 2 — SFTP mở pane Remote tại Home Dir (Priority: P2) — plan T4

**Mục tiêu**: Pane Remote nạp `ResolveRemoteDir(homeDir, Getwd())`; nạp lỗi → về thư mục mặc định và
status đỏ `Home dir not found: <dir>`; không Home Dir → y hệt hiện tại.

**Kiểm thử độc lập**: quickstart.md bước 3, 4, 5, 7 (phần SFTP).

- [X] T008 [US2] Trong internal/adapters/ui/sftp_view.go: thêm trường `homeDir string` vào `fileManager`; `handleFileManager` gọi `t.showFileManager(server, sess)`; đổi chữ ký `showFileManager(server domain.Server, sess ports.SFTPSession)` với `alias := server.Alias` và `fm.homeDir = server.HomeDir`; thay khối khởi tạo pane Remote bằng: `wd` từ `Getwd()` (lỗi → `/`), `start := services.ResolveRemoteDir(fm.homeDir, wd)`, `err = fm.remote.Load(start)`, nếu `err != nil && start != wd` thì `fm.fail("Home dir not found: " + fm.homeDir)` rồi `err = fm.remote.Load(wd)`, nếu `err != nil` thì `fm.fail(err.Error())`. Chạy `go build ./...`, `go test -race ./...`, `gofmt -l .`, `go vet ./...`; xác nhận PASS
- [X] T009 [US2] Commit `feat: open sftp remote pane in the server home dir` (sftp_view.go, tasks.md)

---

## Phase 5: User Story 3 — Cấu hình và xem Home Dir trong giao diện (Priority: P2) — plan T3

**Mục tiêu**: Ô `Home Dir:` trong form Add/Edit (sau `Tags:`), có help/placeholder, trim khi lưu; pane
chi tiết hiện `Home Dir: <giá trị>` khi khác rỗng.

**Kiểm thử độc lập**: `go test -race ./internal/adapters/ui/ -run 'ServerDetails|FieldHelp' -v`;
quickstart.md bước 1, 7, 9.

- [X] T010 [US3] Trong internal/adapters/ui/server_details_test.go thêm `TestServerDetails_HomeDir`: `UpdateServer` với `HomeDir: "/var/www"` → `GetText(false)` chứa `Home Dir: [white]/var/www[-]`; không có HomeDir → không chứa `Home Dir`. Trong internal/adapters/ui/field_help_test.go thêm kiểm `GetFieldHelp("HomeDir") != nil` và `GetFieldPlaceholder("HomeDir") == "e.g. /var/www/app"`. Chạy; xác nhận FAIL
- [X] T011 [US3] Trong internal/adapters/ui/server_details.go thêm group `{name: "lazyssh", fields: []fieldEntry{{"Home Dir", server.HomeDir}}}` vào cuối `groups`. Trong internal/adapters/ui/field_help.go thêm `fieldHelpData["HomeDir"]` (Field `HomeDir`; Description `Remote directory to start in after SSH/SFTP connect. Stored by lazyssh only, never written to ssh config.`; Syntax `path | ~/path`; Examples `/var/www/app`, `~/projects`; Default `none`; Category `lazyssh`). Trong internal/adapters/ui/defaults.go thêm `case "HomeDir": return "e.g. /var/www/app"` vào `GetFieldPlaceholder`. Trong internal/adapters/ui/server_form.go: thêm `HomeDir string` vào `ServerFormData`; Edit: `HomeDir: sf.original.HomeDir`; Add: `HomeDir: ""`; sau dòng `Tags:` gọi `sf.addInputFieldWithHelp(form, "Home Dir:", "HomeDir", defaultValues.HomeDir, 40, GetFieldPlaceholder("HomeDir"))`; thu thập `HomeDir: getFieldText("Home Dir:")`; dựng `domain.Server` với `HomeDir: strings.TrimSpace(data.HomeDir)`. Chạy `go build ./...`, `go test -race ./...`, `gofmt -l .`, `go vet ./...`; xác nhận PASS; kiểm thủ công quickstart bước 1 nếu có terminal
- [X] T012 [US3] Commit `feat: add home dir field to server form and details` (server_form.go, server_details.go, server_details_test.go, field_help.go, field_help_test.go, defaults.go, tasks.md)

---

## Phase 6: Hoàn thiện — plan T5

- [X] T013 [P] Cập nhật README.md mục "Server Management": gạch đầu dòng `📁 Set a \`Home Dir:\` per server (e.g. \`/var/www/app\`, \`~/projects\`): SSH opens the shell there and the SFTP file manager opens the remote pane there. Stored in \`~/.lazyssh/metadata.json\` (synced), never in \`~/.ssh/config\`.`
- [X] T014 Chạy `make build` và `make test`; xác nhận PASS; commit `docs: document per-server home dir` (README.md, specs/008-profile-home-dir/tasks.md)

---

## Phụ thuộc & thứ tự

- Phase 2 → Phase 3 (dùng `HomeDir`, `RemoteCommand`) → Phase 4 (dùng `ResolveRemoteDir` từ T005)
  → Phase 5 → Phase 6.
- Phase 5 chỉ cần Phase 2; có thể làm trước Phase 4 nếu muốn kiểm thủ công US1 sớm (cần form để nhập
  Home Dir; tạm thời có thể sửa tay `~/.lazyssh/metadata.json` thêm `"home_dir"`).
- T013 độc lập về file, có thể làm song song với Phase 4/5.

## Ví dụ chạy song song

```text
Phase 4 (sftp_view.go) ‖ Phase 5 (server_form.go, server_details.go, field_help.go, defaults.go) ‖ T013 (README.md)
```

## Chiến lược hiện thực

- **MVP**: Phase 2 + Phase 3 (US1) + Phase 5 (US3): đặt Home Dir trong form và SSH vào thẳng đó.
- Sau đó Phase 4 (US2, SFTP) rồi tài liệu. Mỗi phase để `go test -race ./...` xanh và một commit.
