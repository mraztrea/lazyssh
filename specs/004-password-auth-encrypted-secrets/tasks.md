---

description: "Danh sách task cho Xác thực bằng mật khẩu và mã hoá bí mật khi lưu trữ"
---

# Tasks: Xác thực bằng mật khẩu và mã hoá bí mật khi lưu trữ

**Input**: Tài liệu thiết kế trong `/specs/004-password-auth-encrypted-secrets/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/askpass.md, quickstart.md

**Tests**: Bắt buộc. Hiến pháp II: viết test, chạy và thấy nó FAIL, hiện thực, thấy PASS, commit.
Test chỉ dùng `t.TempDir()` và sinh mọi mật khẩu và key lúc chạy (`crypto/rand`, hoặc
`ed25519.GenerateKey` + `ssh.MarshalPrivateKey` cho nội dung key, như test của spec 001); không có
bí mật viết cứng trong fixture. Phần nối tview (T037–T038) được kiểm thủ công theo quickstart.md.

**Organization**: Mỗi phase ứng với một task trong plan.md ("Thiết kế theo task", T1–T7); mỗi phase
kết thúc bằng một commit với message kết thúc bằng dòng `Co-Authored-By` của phiên. US2 (mã hoá)
chạy trước US1 (đăng nhập bằng mật khẩu) vì US1 không bao giờ được lưu mật khẩu chưa mã hoá; cả hai
đều P1.

## Format: `[ID] [P?] [Story] Mô tả`

- **[P]**: Có thể chạy song song (file khác nhau, không phụ thuộc)
- **[Story]**: US1 đăng nhập bằng mật khẩu, US2 mã hoá khi lưu, US3 auth method trên form, US4 sync

## Phase 1: Setup

Không có task setup: module, công cụ lint và test đã có; không thêm dependency (chỉ stdlib).

---

## Phase 2: Nền tảng (Foundational) — plan T1 `internal/secretbox`

**Mục tiêu**: Seal/open từng giá trị bằng AES-256-GCM với file key 32 byte.

**Kiểm thử độc lập**: `go test ./internal/secretbox/ -v`

- [X] T001 Viết test thất bại trong file mới internal/secretbox/secretbox_test.go (Apache header; key = 32 byte từ `crypto/rand`; plaintext sinh lúc chạy): `TestSealOpen_RoundTrip`, `TestSeal_FreshNonceEachTime` (hai lần seal cùng giá trị cho kết quả khác nhau), `TestSeal_NoPlaintextInOutput` (đầu ra bắt đầu bằng `enc:v1:` và không chứa plaintext), `TestOpen_TamperedFails` (đảo một byte ciphertext → `errors.Is(err, ErrDecrypt)`), `TestOpen_WrongKeyFails` (`ErrDecrypt`), `TestOpen_LegacyPlaintextPassthrough` (không có tiền tố `enc:` → trả nguyên, cho phép key nil), `TestOpen_UnknownVersion` (`enc:v9:...` → `ErrUnknownFormat`), `TestLoadOrCreateKey_Creates0600In0700Dir` (đường dẫn `<tmp>/sub/master.key`; 32 byte; file 0600, thư mục 0700), `TestLoadOrCreateKey_KeepsExisting` (lần gọi thứ hai trả đúng các byte cũ; file có sẵn 5 byte → lỗi corrupt và byte file không đổi), `TestLoadKey_MissingNamesPath` (chuỗi lỗi chứa đường dẫn, `errors.Is(err, fs.ErrNotExist)`), `TestLoadKey_WrongSizeIsCorrupt` (lỗi chứa đường dẫn và `corrupt`); mọi thông báo lỗi được kiểm là không chứa plaintext
- [X] T002 Chạy `go test ./internal/secretbox/ -v`; xác nhận FAIL (chưa có package)
- [X] T003 Tạo internal/secretbox/secretbox.go (Apache header, chỉ stdlib): `ErrDecrypt`, `ErrUnknownFormat`, `LoadKey`, `LoadOrCreateKey` (`MkdirAll(dir, 0o700)`, `O_WRONLY|O_CREATE|O_EXCL` 0600, gặp `fs.ErrExist` thì `LoadKey` lại; không bao giờ ghi đè), `Seal` (`enc:v1:` + base64.StdEncoding(nonce[12] || gcm.Seal)), `Open` (không có tiền tố `enc:` → trả nguyên; tiền tố khác `enc:v1:` → `ErrUnknownFormat`; lỗi decode/độ dài/xác thực → `ErrDecrypt`), `IsSealed` — đúng như plan.md T1 và data-model.md mục "Giá trị đã mã hoá" / "Master key"
- [X] T004 Chạy `make test` (`go test -race ./...`); xác nhận PASS
- [X] T005 Commit `feat: add secretbox for encrypting secrets at rest` (internal/secretbox)

---

## Phase 3: User Story 2 — Bí mật trong metadata.json được mã hoá (Priority: P1) — plan T2

**Mục tiêu**: `private_key` và `password` mới ở dạng `enc:v1:` trên đĩa, plaintext trong bộ nhớ;
plaintext cũ được chuyển đổi ở lần ghi kế tiếp; dữ liệu không giải mã được thì thất bại đóng với
lỗi nêu tên file.

**Kiểm thử độc lập**: `go test ./internal/adapters/data/ssh_config_file/ -run 'Metadata|Password' -v`

- [X] T006 [US2] Viết test thất bại trong file mới internal/adapters/data/ssh_config_file/metadata_manager_test.go (Apache header, home trong `t.TempDir()`, `NewRepository(zap.NewNop().Sugar(), cfg, meta)`, nội dung key và mật khẩu sinh lúc chạy): `TestMetadata_SecretsEncryptedOnDisk` (lưu server có `PrivateKey` và `Password`; file thô không chứa cái nào, cả hai trường bắt đầu bằng `enc:v1:`; `master.key` là 0600), `TestMetadata_RoundTripPasswordAndKey` (ListServers trả đúng giá trị gốc), `TestMetadata_NoSecretsNoMasterKey` (lưu server không có bí mật → không có `master.key`), `TestMetadata_LegacyPlaintextMigratedOnNextWrite` (ghi thô `{"a":{"password":"<sinh ngẫu nhiên>"}}`; ListServers trả nó; `SetPinned("a", true)`; file thô giờ có `enc:v1:` và không còn mật khẩu), `TestMetadata_TamperedValueFailsLoadWithoutWrite` (đảo một byte của giá trị mã hoá; lỗi ListServers chứa `metadata.json` và `"a"`; `SetPinned` thất bại; byte file không đổi), `TestMetadata_MissingMasterKeyFailsWithoutWrite` (xoá `master.key`; lỗi chứa `master.key`; không có `master.key` mới; byte metadata không đổi), `TestMetadata_ErrorsNeverContainSecret` (lỗi từ hai test thất bại trên không chứa mật khẩu hay bất kỳ dòng key nào), `TestRepo_GetPassword` (đã lưu → giá trị; alias không tồn tại → `""`, nil; sau lời gọi: byte `metadata.json` không đổi, không có file mới trong `keys/`, không tạo `master.key` — FR-015), `TestRepo_RenameMovesPassword` (UpdateServer với alias mới giữ mật khẩu chỉ dưới alias mới)
- [X] T007 [US2] Chạy `go test ./internal/adapters/data/ssh_config_file/ -run 'Metadata|Password' -v`; xác nhận FAIL
- [X] T008 [P] [US2] Thêm `Password string // never log; sealed at rest in metadata.json` sau `PrivateKey` trong internal/core/domain/server.go
- [X] T009 [P] [US2] Thêm `GetPassword(alias string) (string, error)` vào `ServerRepository` trong internal/core/ports/repositories.go
- [X] T010 [US2] Trong internal/adapters/data/ssh_config_file/metadata_manager.go: `ServerMetadata.Password string \`json:"password,omitempty"\``; `metadataManager.keyPath = filepath.Join(filepath.Dir(filePath), "master.key")`; `loadAll` mở `PrivateKey`/`Password` đã mã hoá (key nạp một lần bằng `secretbox.LoadKey`, chỉ khi có giá trị `IsSealed`), bọc lỗi thành `fmt.Errorf("decrypt %s of %q in %s: %w", field, alias, m.filePath, err)`; hàm mới `marshalSealed(map[string]ServerMetadata) ([]byte, error)` (chép map, chỉ `LoadOrCreateKey` khi có bí mật không rỗng, `Seal` từng cái, `MarshalIndent`) được `saveAll` dùng; `updateServer` đặt `merged.Password = server.Password`; không bao giờ log giá trị
- [X] T011 [US2] Trong `mergeMetadata` của internal/adapters/data/ssh_config_file/mapper.go: chép `meta.Password` vào `server.Password`
- [X] T012 [US2] Trong internal/adapters/data/ssh_config_file/ssh_config_file_repo.go: `ListServers` trả `fmt.Errorf("load metadata: %w", err)` thay vì warn + map rỗng; thêm `GetPassword(alias)` = `metadataManager.loadAll()` → `meta[alias].Password` (không parse config, không ghi file key, không ghi gì)
- [X] T013 [US2] Trong `loadInitialData` của internal/adapters/ui/tui.go và `refreshServerList` của internal/adapters/ui/handlers.go: khi `ListServers` lỗi gọi `t.showStatusTempColor(err.Error(), "#FF6B6B")` (FR-009)
- [X] T014 [US2] Chạy `make test`; xác nhận PASS (test crud/key_store có sẵn không đổi)
- [X] T015 [US2] Commit `feat: encrypt embedded keys and passwords in metadata.json` (internal/core/domain internal/core/ports internal/adapters/data/ssh_config_file internal/adapters/ui/tui.go internal/adapters/ui/handlers.go)

---

## Phase 4: User Story 1 — Đăng nhập bằng mật khẩu đã lưu: phía khởi chạy (Priority: P1) — plan T4

**Mục tiêu**: ssh/sftp cho server có mật khẩu chạy với env askpass và tuỳ chọn xác thực; server
khác giống hệt từng byte như hiện nay.

**Kiểm thử độc lập**: `go test ./internal/core/services/ -run 'PasswordAuth|SSHCmd|SFTP|Validate' -v`

- [X] T016 [US1] Viết test thất bại trong file mới internal/core/services/askpass_test.go (Apache header; thay `executable` để trả `/opt/lazyssh`; fake lookup trả mật khẩu sinh ngẫu nhiên): `TestPasswordAuth_NoPasswordUnchanged` (args nil, env nil), `TestPasswordAuth_SetsAskpassEnvAndOptions` (args đúng là `-o PreferredAuthentications=password,keyboard-interactive -o NumberOfPasswordPrompts=1`; env chứa `SSH_ASKPASS=/opt/lazyssh`, `SSH_ASKPASS_REQUIRE=force`, `LAZYSSH_ASKPASS_ALIAS=<alias>`), `TestPasswordAuth_SecretNotInArgsOrEnv`, `TestPasswordAuth_LookupErrorAborts`, `TestSSHCmd_OptionsBeforeDestination` (kết nối `ssh <opts> <alias>`; kèm tham số `ssh <extra> <opts> <alias>`; forward `ssh <extra> <opts> -N <alias>`; biến thể không mật khẩu có args như hiện nay và `Env == nil`)
- [X] T017 [P] [US1] Thêm test thất bại vào internal/core/services/server_service_test.go: `TestValidateServer_PasswordNewlineRejected` (`"a\nb"` và `"a\rb"` → `password must be a single line`; `" spaced "` được chấp nhận nguyên vẹn)
- [X] T018 [P] [US1] Thêm test thất bại `TestSFTPCmd_PasswordServer` vào internal/core/services/sftp_service_test.go (builder mặc định có auth args → `ssh <opts> -s -- <alias> sftp`; không có → `ssh -s -- <alias> sftp`); sửa các lời gọi `newSFTPServiceWithCmd` có sẵn để truyền lookup trả `"", nil` và cho `helperCmd` nhận rồi bỏ qua `authArgs`
- [X] T019 [US1] Chạy `go test ./internal/core/services/ -v`; xác nhận FAIL
- [X] T020 [US1] Tạo internal/core/services/askpass.go (Apache header): `var executable = os.Executable`; `passwordAuth(lookup func(string) (string, error), alias string) (args, env []string, err error)` theo contracts/askpass.md mục "Khởi chạy"
- [X] T021 [US1] Trong internal/core/services/server_service.go: thêm `sshCmd(alias string, pre, post []string) (*exec.Cmd, error)` gọi `passwordAuth(s.serverRepository.GetPassword, alias)` và dựng `ssh <pre> <authArgs> <post> <alias>` với `Env = env`; dùng nó trong `SSH` (pre nil, post nil), `SSHWithArgs` (pre = extra), `StartForward` (pre = extra, post = `-N`; bỏ việc nối `-N alias` vào extraArgs); `validateServer` từ chối `\r`/`\n` trong `Password` với `password must be a single line`
- [X] T022 [US1] Trong internal/core/services/sftp_service.go: `NewSFTPService(logger, password func(string) (string, error))`, `newSFTPServiceWithCmd(logger, password, cmdFor func(alias string, authArgs []string) *exec.Cmd)`; builder mặc định `ssh <authArgs> -s -- <alias> sftp`; `Connect` gọi `passwordAuth`, trả lỗi của nó, đặt `cmd.Env = env`
- [X] T023 [US1] Trong cmd/main.go truyền `serverRepo.GetPassword` cho `services.NewSFTPService`
- [X] T024 [US1] Chạy `make test`; xác nhận PASS
- [X] T025 [US1] Commit `feat: launch ssh and sftp with askpass for password servers` (internal/core/services cmd/main.go)

---

## Phase 5: User Story 1 — Đăng nhập bằng mật khẩu đã lưu: phía helper (Priority: P1) — plan T5

**Mục tiêu**: lazyssh khởi động với `LAZYSSH_ASKPASS_ALIAS` chỉ trả lời prompt mật khẩu của server
đích, chuyển tiếp prompt khác tới `/dev/tty`, còn lại thoát mã khác 0 và không in gì.

**Kiểm thử độc lập**: `go test ./internal/core/services/ -run 'Askpass|ParseSSHG' -v`

- [X] T026 [US1] Thêm test thất bại vào internal/core/services/askpass_test.go (fake cho lookup, `userHost` trả `("alice","10.0.0.5")`, và tty là một `io.ReadWriteCloser` trong bộ nhớ): `TestAskpass_PrintsPasswordForTargetPrompt` (`"alice@10.0.0.5's password: "` → stdout `<pw>\n`, thoát 0), `TestAskpass_KeyboardInteractivePromptAnswered` (`"(alice@10.0.0.5) Password: "`), `TestAskpass_OtherHostPromptNotAnswered` (`"bob@jump's password: "` → được chuyển tiếp: tty thấy prompt, stdout là dòng từ tty, mật khẩu không bao giờ được in), `TestAskpass_HostKeyPromptRelayed` (`"Are you sure you want to continue connecting (yes/no/[fingerprint])? "` + tty `yes\n` → stdout `yes\n`, thoát 0), `TestAskpass_NoTTYExitsNonZero` (mở tty lỗi → thoát 1, stdout rỗng), `TestAskpass_UnknownAliasExitsNonZero` và `TestAskpass_NoPasswordExitsNonZero` (lookup `""` → thoát 1, stdout rỗng), `TestAskpass_LookupErrorExitsNonZeroWithoutSecret`, `TestAskpass_UserHostErrorRelays` (resolver lỗi → xử lý như prompt khác); `TestParseSSHG_UserHostKeyAlias` trong internal/core/services/server_service_test.go (văn bản `ssh -G` mẫu: `hostkeyalias` thắng `hostname`; bỏ qua `hostkeyalias none`)
- [X] T027 [US1] Chạy `go test ./internal/core/services/ -run 'Askpass|ParseSSHG' -v`; xác nhận FAIL
- [X] T028 [US1] Trong internal/core/services/askpass.go thêm `Askpass(lookup func(string) (string, error), userHost func(string) (string, string, error), alias, prompt string, out io.Writer, tty func() (io.ReadWriteCloser, error)) int` theo bảng "Helper" của contracts/askpass.md, kèm comment `// ponytail: relayed input echoes (no x/term); fine for host-key yes/no and OTP codes, upgrade if passphrases ever reach the helper.`; thông báo stderr chỉ nêu alias/file
- [X] T029 [US1] Trong internal/core/services/server_service.go tách `parseSSHG(out string) map[string]string` ra khỏi `resolveSSHDestination` (giữ nguyên hành vi) và thêm hàm exported `ResolveUserHost(alias string) (user, host string, err error)` (`ssh -G`; `hostkeyalias` trừ khi rỗng/`none`, nếu không thì `hostname`)
- [X] T030 [US1] Trong cmd/main.go: câu lệnh đầu tiên của `main()` = `if alias := os.Getenv("LAZYSSH_ASKPASS_ALIAS"); alias != "" { os.Exit(runAskpass(alias)) }`; `runAskpass` lấy home, dựng `ssh_config_file.NewRepository(zap.NewNop().Sugar(), cfg, meta)`, prompt = `os.Args[1]` nếu có, hàm mở tty là `os.OpenFile("/dev/tty", os.O_RDWR, 0)`, trả `services.Askpass(repo.GetPassword, services.ResolveUserHost, alias, prompt, os.Stdout, opener)`; không logger, không TUI
- [ ] T031 [US1] Chạy `make test`; rồi kiểm thủ công theo quickstart.md bước 3–5 với sshd cục bộ dùng mật khẩu — `make test` PASS; bước thủ công 3–5 CHƯA kiểm (máy không có sshd nhận mật khẩu)
- [X] T032 [US1] Commit `feat: run lazyssh as ssh askpass helper for stored passwords` (internal/core/services cmd/main.go)

---

## Phase 6: User Story 4 — Sync mang theo mật khẩu và key (Priority: P2) — plan T3

**Mục tiêu**: Export giải mã vào bundle v1 không đổi; Import mã hoá lại bằng key cục bộ.

**Kiểm thử độc lập**: `go test ./internal/adapters/data/ssh_config_file/ -run 'Export|Import' -v`

- [X] T033 [US4] Thêm test thất bại vào internal/adapters/data/ssh_config_file/snapshot_test.go: `TestExport_DecryptsSecrets` (metadata trong bundle có `password`/`private_key` plaintext), `TestExport_FailsOnUndecryptable` (thiếu `master.key` → lỗi nêu tên nó), `TestImport_SealsWithLocalKey` (file thô chỉ có `enc:v1:`; ListServers trả giá trị gốc), `TestImport_CreatesMasterKeyIfMissing`, `TestImport_RejectsSealedValuesBeforeWrite` (giá trị bundle `enc:v1:...` → lỗi `sync bundle contains encrypted values from another machine`; config và metadata không bị đụng, không có backup), `TestExportImport_DifferentMasterKeys` (hai home tạm); đổi `TestImport_WritesBothAndBacksUpConfig` sang so sánh file metadata dạng JSON đã decode (Import nay ghi có thụt lề)
- [X] T034 [US4] Chạy `go test ./internal/adapters/data/ssh_config_file/ -run 'Export|Import' -v`; xác nhận FAIL
- [X] T035 [US4] Trong internal/adapters/data/ssh_config_file/snapshot.go: `Export` dùng `metadataManager.loadAll()` + `json.Marshal` (gọn; rỗng → `{}`), trả lỗi nạp; `Import` (sau kiểm version, trước `saveRawConfig`) unmarshal `b.Metadata` vào `map[string]ServerMetadata`, từ chối mọi bí mật `secretbox.IsSealed`, rồi ghi `marshalSealed(meta)` qua tmp + rename có sẵn; sau đó `make test`, xác nhận PASS; commit `feat: keep sync bundle plaintext inside, re-encrypt on import` (internal/adapters/data/ssh_config_file)

---

## Phase 7: User Story 3 — Chọn phương thức xác thực trên form (Priority: P2) — plan T6

**Mục tiêu**: Dropdown `Auth method:` + `Password:` có che ký tự; bí mật không được chọn bị xoá khi
lưu; dấu trên details pane.

**Kiểm thử độc lập**: `go test ./internal/adapters/ui/ -run 'AuthMethod|Details' -v`, rồi
quickstart.md bước 1

- [X] T036 [US3] Viết test thất bại trong file mới internal/adapters/ui/auth_method_test.go (Apache header): `TestAuthMethodOf` (có password → `password`; chỉ có key → `embedded key`; không có gì → `identity file`), `TestApplyAuthMethod_PasswordClearsKey` (`PrivateKey` và `PrivateKeyImportPath` bị xoá, `Password` giữ nguyên kể cả khoảng trắng hai đầu), `TestApplyAuthMethod_PasswordRequired` (rỗng → `password is required`), `TestApplyAuthMethod_EmbeddedKeyClearsPassword`, `TestApplyAuthMethod_IdentityFileClearsBoth`, `TestAuthFieldsEnabled` (`identity file` → key tắt, password tắt; `embedded key` → key bật, password tắt; `password` → key tắt, password bật); cùng `TestServerDetails_PasswordNeverRendered` (nội dung details chứa `(password)` và không chứa mật khẩu sinh ngẫu nhiên) trong file mới internal/adapters/ui/server_details_test.go (Apache header); chạy, xác nhận FAIL
- [X] T037 [US3] Tạo internal/adapters/ui/auth_method.go (Apache header): hằng `authIdentityFile = "identity file"`, `authEmbeddedKey = "embedded key"`, `authPassword = "password"`; `authMethodOf(domain.Server) string`; `applyAuthMethod(*ServerFormData) error` theo data-model.md mục "ServerFormData"; `authFieldsEnabled(method string) (key, password bool)`; trong internal/adapters/ui/server_details.go nối `" (password)"` vào dòng key khi `server.Password != ""`
- [X] T038 [US3] Trong internal/adapters/ui/server_form.go: `ServerFormData` + `AuthMethod`, `Password`; `getDefaultValues` điền chúng từ `sf.original` (`authMethodOf`); `createAuthenticationForm` bắt đầu bằng dropdown `Auth method:`, giữ các item embedded key, thêm `Password:` (`tview.NewInputField().SetLabel("Password:").SetMaskCharacter('*')`, giá trị mặc định, help key `Password`), và hàm selected của dropdown gọi `SetDisabled` theo `authFieldsEnabled` để chỉ các trường của phương thức được chọn được bật (cũng áp dụng một lần khi build); `getFormData` đọc cả hai; `handleSave` gọi `applyAuthMethod` trước `resolvePrivateKey` (lỗi → cùng modal như lỗi import); `dataToServer` chép `Password`; `hasUnsavedChanges` so sánh `Password`; trong internal/adapters/ui/field_help.go thêm mục `AuthMethod` và `Password` (không có ví dụ trông giống bí mật); chạy `make test`, xác nhận PASS; kiểm thủ công quickstart.md bước 1; commit `feat: add auth method selector and password field to server form` (internal/adapters/ui)

---

## Phase 8: Hoàn thiện & các vấn đề xuyên suốt (Polish & Cross-Cutting) — plan T7

- [X] T039 [P] Cập nhật README.md: xác thực mật khẩu (OpenSSH ≥ 8.4; client cũ hơn sẽ hỏi), `~/.lazyssh/master.key` (riêng từng máy, không sync, hãy sao lưu; nếu mất thì Pull từ sync hoặc nhập lại bí mật), `Auth method:` trên form
- [X] T040 Rà soát bí mật: `grep -rnE 'Infow|Errorw|Warnw|Debugw|Warnf|Errorf' internal cmd` và xác nhận không lời gọi nào truyền giá trị `domain.Server`, `ServerMetadata`, `Password` hay `PrivateKey`; `grep -rn` toàn repo tìm header private key được commit ngoài mã test sinh lúc chạy; xác nhận `matchesQuery` (internal/adapters/data/ssh_config_file/crud.go) và lệnh copy SSH (internal/adapters/ui/utils.go) không đọc `Password`/`PrivateKey` (FR-018); sửa mọi chỗ trúng
- [ ] T041 Chạy `make build` và `make test` (`go test -race ./...`); chạy toàn bộ bước thủ công 1–8 trong quickstart.md; commit `docs: document password auth and encrypted secrets` (README.md) — `make test` PASS, commit xong; `make build` dừng ở lint do golangci-lint v1.64.2 không đọc được export data Go 1.26 (lỗi có sẵn); bước thủ công 1, 2, 6, 7 đã kiểm, 3–5 và 8 CHƯA kiểm (không có sshd)

---

## Phụ thuộc & thứ tự thực hiện

- Phase 2 (secretbox) → Phase 3 (US2: lưu trữ + `GetPassword`) → Phase 4 (US1 khởi chạy) → Phase 5
  (US1 helper). Phase 6 (US4) chỉ cần Phase 3. Phase 7 (US3) cần Phase 3 (trường `Password`) và
  Phase 4 (`validateServer`). Phase 8 cuối cùng.
- Trong một phase: test → thấy fail → hiện thực → thấy pass → commit.

### Cơ hội song song

- T008/T009 (domain và ports); T017/T018 (file test khác nhau).
- Sau Phase 3: Phase 6 (US4, file adapter) chạy song song được với Phase 4–5 (services, cmd);
  Phase 7 (file UI) chạy song song được với Phase 5 khi Phase 4 đã xong.

## Chiến lược hiện thực

MVP = Phase 2–5 + 7: người dùng nhập được mật khẩu trên form, nó được lưu mã hoá, và kết nối/SFTP
đăng nhập không cần hỏi. Phase 6 giữ sync hoạt động với cách lưu mới (phải phát hành cùng bản: thiếu
nó thì Push sẽ export giá trị `enc:v1:` mà máy khác không mở được). Phase 8 khép lại.

## Truy vết (Traceability)

| Spec | Tasks |
|---|---|
| FR-001, FR-002, FR-003, FR-004 | T036–T038 (phần xuống dòng của FR-004 còn ở T017, T021) |
| FR-005 | T006, T008, T010, T011 |
| FR-006, FR-007 | T001–T003, T006, T010 |
| FR-008 | T006, T010, T012 |
| FR-009 | T013 |
| FR-010 | mã spec 001 không đổi; được T014 bao phủ (test key_store có sẵn) |
| FR-011, FR-012 | T016, T018, T020–T023 |
| FR-013, FR-014, FR-015 | T026, T028–T030 |
| FR-016, FR-017 | T033, T035 |
| FR-018 | T001, T006, T016, T026, T036, T040 |
| SC-001 | T031, T041 |
| SC-002, SC-003 | T006 |
| SC-004 | T033 |
| SC-005 | T016, T026, T036, T040 |
| SC-006 | T016 |
