# Kế hoạch hiện thực: Xác thực bằng mật khẩu và mã hoá bí mật khi lưu trữ

**Branch**: `004-password-auth-encrypted-secrets` | **Ngày**: 2026-10-29 | **Spec**: [spec.md](./spec.md)

**Input**: Đặc tả tính năng từ `/specs/004-password-auth-encrypted-secrets/spec.md`

## Tóm tắt

Thêm `Password` cho từng server và mã hoá cả `private_key` lẫn `password` trong
`~/.lazyssh/metadata.json` bằng AES-256-GCM với `~/.lazyssh/master.key` riêng từng máy. Phần crypto
là package lá chỉ dùng stdlib `internal/secretbox`, chỉ data adapter dùng, nên mọi thứ phía trên
adapter vẫn thấy plaintext. `ssh` hệ thống nhận mật khẩu bằng cách chạy chính lazyssh làm
`SSH_ASKPASS` (`SSH_ASKPASS_REQUIRE=force`, `LAZYSSH_ASKPASS_ALIAS=<alias>`); helper chỉ trả lời
prompt mật khẩu của đúng `user@host` đích và chuyển tiếp mọi prompt khác tới `/dev/tty`. Sync giữ
bundle version 1: Export giải mã, Import mã hoá lại bằng key cục bộ. Form có thêm dropdown
`Auth method:` bật các trường tương ứng và xoá bí mật của phương thức kia khi lưu.

## Bối cảnh kỹ thuật (Technical Context)

**Language/Version**: Go 1.26 (`go 1.26.0` trong go.mod)

**Primary Dependencies**: `tview`/`tcell`, `zap`, `cobra`, `pkg/sftp` có sẵn; stdlib
`crypto/aes`, `crypto/cipher`, `crypto/rand`, `encoding/base64`. Không thêm module mới.

**Storage**: `~/.lazyssh/metadata.json` (có sẵn, 0600; hai trường đổi dạng, một trường mới),
`~/.lazyssh/master.key` (mới, 32 byte, 0600), `~/.lazyssh/keys/<alias>` không đổi (plaintext,
OpenSSH đọc nó)

**Testing**: `go test -race ./...` (`make test`); home trong `t.TempDir()`; mật khẩu và key sinh
lúc chạy; việc khởi chạy ssh được test bằng cách kiểm `*exec.Cmd` dựng ra (args, env), helper được
test như hàm thuần với fake cho lookup / resolver / tty

**Target Platform**: Linux và macOS với OpenSSH ≥ 8.4 cho đường không hỏi mật khẩu; OpenSSH cũ hơn
và Windows lùi về prompt của chính ssh

**Project Type**: công cụ CLI TUI trên desktop

**Performance Goals**: một thao tác AES-GCM cho mỗi bí mật mỗi lần nạp/lưu (micro giây); helper tốn
thêm một lần gọi `ssh -G` cho mỗi prompt mật khẩu (vài chục ms)

**Constraints**: mật khẩu không bao giờ trong argv, env, log, status bar, details pane, thông báo
lỗi, fixture; thất bại đóng khi dữ liệu không giải mã được (không ghi); không thêm dependency; server
không có mật khẩu khởi chạy giống hệt từng byte

**Scale/Scope**: 7 task kế hoạch; 1 package mới (`internal/secretbox`), 8 file mới (3 mã nguồn:
`secretbox.go`, `askpass.go`, `auth_method.go`; 5 test), ~12 file sửa

## Kiểm tra hiến pháp (Constitution Check)

*GATE: Phải qua trước Phase 0 research. Kiểm lại sau Phase 1 design.*

| Nguyên tắc | Kiểm tra | Trạng thái |
|---|---|---|
| I. Phân lớp hexagonal | `internal/secretbox` chỉ import stdlib và chỉ được `adapters/data/ssh_config_file` import (tiền lệ: `internal/logger`), nên data adapter không bao giờ import `services` (research R1). Domain chỉ thêm một trường string. Services lấy mật khẩu qua `ports.ServerRepository` có sẵn (+`GetPassword`). UI chỉ dùng `ports`. `cmd/main.go` nối điểm vào askpass: repository + `services.Askpass`. | PASS |
| II. Test trước | Mỗi task bắt đầu bằng test thất bại có tên cụ thể: round-trip/tamper/plaintext cũ/file key của crypto (T1), mã hoá trên đĩa + chuyển đổi + thất bại đóng (T2), Export/Import giữa hai master key (T3), dựng env/args và bất biến không lộ bí mật (T4), khớp prompt / chuyển tiếp / mã thoát của askpass (T5), `applyAuthMethod` của form + dấu ở details pane (T6). Mọi test đụng filesystem dùng `t.TempDir()`. | PASS |
| III. Ít dependency | Chỉ stdlib; không `golang.org/x/term` (đầu vào chuyển tiếp bị echo, giới hạn ở research R6), không `sshpass`, không SSH client nội bộ. | PASS |
| IV. An toàn bí mật | `master.key` 0600 trong thư mục 0700, tạo bằng `O_EXCL`; bí mật được mã hoá khi lưu; lỗi chỉ nêu file và alias; helper không ghi file và không tạo logger; mật khẩu chỉ đi qua pipe stdout của helper tới ssh; kiểm `user@host` ngăn gửi nó tới host `ProxyJump`; details pane chỉ hiện dấu `(password)`. | PASS |
| V. Đơn giản (YAGNI) | Không có interface mới (chỉ thêm một method vào port có sẵn); không lưu trường auth-method; không xoay vòng key, không keyring, không master password; một package lá với năm hàm. Các giới hạn được đánh dấu `ponytail:` (đầu vào chuyển tiếp bị echo; lùi về khi OpenSSH trước 8.4). | PASS |

Kiểm lại sau thiết kế: PASS; Complexity Tracking trống.

## Cấu trúc dự án (Project Structure)

### Tài liệu (tính năng này)

```text
specs/004-password-auth-encrypted-secrets/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── askpass.md
├── checklists/requirements.md
└── tasks.md
```

### Mã nguồn (gốc repository)

```text
internal/secretbox/secretbox.go, secretbox_test.go           # MỚI (T1)
internal/core/domain/server.go                               # + Password (T2)
internal/core/ports/repositories.go                          # + ServerRepository.GetPassword (T2)
internal/adapters/data/ssh_config_file/
├── metadata_manager.go                                      # trường Password; seal/open trong saveAll/loadAll; keyPath (T2)
├── metadata_manager_test.go                                 # MỚI (T2)
├── mapper.go                                                # mergeMetadata chép Password (T2)
├── ssh_config_file_repo.go                                  # ListServers trả lỗi metadata; GetPassword (T2)
├── snapshot.go, snapshot_test.go                            # Export giải mã, Import mã hoá (T3)
internal/core/services/
├── askpass.go, askpass_test.go                              # MỚI passwordAuth, Askpass (T4, T5)
├── server_service.go, server_service_test.go                # validateServer chặn xuống dòng; sshCmd; ResolveUserHost (T4, T5)
├── sftp_service.go, sftp_service_test.go                    # NewSFTPService(logger, password lookup); cmd nhận auth (T4)
cmd/main.go                                                  # nhánh askpass đầu main; NewSFTPService(log, serverRepo.GetPassword) (T4, T5)
internal/adapters/ui/
├── server_form.go                                           # dropdown Auth method, trường Password (T6)
├── auth_method.go, auth_method_test.go                      # MỚI applyAuthMethod + authMethodOf (T6)
├── server_details.go, server_details_test.go                # dấu "(password)"; test MỚI (T6)
├── field_help.go                                            # help cho AuthMethod, Password (T6)
├── tui.go, handlers.go                                      # hiện lỗi ListServers trên status bar (T2)
README.md                                                    # xác thực mật khẩu + mã hoá khi lưu (T7)
```

**Structure Decision**: giữ bố cục hexagonal hiện có; một package lá mới `internal/secretbox`.

## Thiết kế theo task

### T1: `internal/secretbox`

- `const prefix = "enc:v1:"`; `var ErrDecrypt = errors.New("cannot decrypt value")`,
  `ErrUnknownFormat = errors.New("unknown encrypted value format")`.
- `LoadKey(path string) ([]byte, error)`: đọc; không tồn tại → `fmt.Errorf("master key %s: %w",
  path, err)`; `len != 32` → `fmt.Errorf("master key %s is corrupt", path)`.
- `LoadOrCreateKey(path string) ([]byte, error)`: `LoadKey`; nếu không tồn tại → `MkdirAll(dir,
  0o700)`, 32 byte từ `crypto/rand`, `OpenFile(O_WRONLY|O_CREATE|O_EXCL, 0o600)`, ghi, đóng; nếu
  việc tạo tranh chấp với tiến trình ghi khác (`ErrExist`) → `LoadKey` lại. Không bao giờ ghi đè.
- `Seal(key []byte, plain string) (string, error)`: nonce 12 byte mới, GCM, `prefix +
  base64(nonce||ct)`.
- `Open(key []byte, v string) (string, error)`: không có tiền tố `enc:` → trả nguyên `v`; có tiền tố
  nhưng không phải `enc:v1:` → `ErrUnknownFormat`; lỗi decode / độ dài / `gcm.Open` → `ErrDecrypt`.
- `IsSealed(v string) bool` = `strings.HasPrefix(v, "enc:")`.
- Test: `TestSealOpen_RoundTrip`, `TestSeal_FreshNonceEachTime`, `TestSeal_NoPlaintextInOutput`,
  `TestOpen_TamperedFails`, `TestOpen_WrongKeyFails`, `TestOpen_LegacyPlaintextPassthrough`,
  `TestOpen_UnknownVersion`, `TestLoadOrCreateKey_Creates0600In0700Dir`,
  `TestLoadOrCreateKey_KeepsExisting`, `TestLoadKey_MissingNamesPath`,
  `TestLoadKey_WrongSizeIsCorrupt`.

### T2: Trường Password và mã hoá trong metadata

- `domain.Server.Password string` (comment: không bao giờ log; được mã hoá khi lưu).
- `ServerMetadata.Password string \`json:"password,omitempty"\``.
- `metadataManager.keyPath = filepath.Join(filepath.Dir(filePath), "master.key")`, đặt trong
  `newMetadataManager`.
- `loadAll`: sau unmarshal, với mỗi mục có `PrivateKey`/`Password` đã mã hoá: nạp key một lần
  (`LoadKey`), `Open`; lỗi → `fmt.Errorf("decrypt %s of %q in %s: %w", field, alias, m.filePath,
  err)` (lỗi key đã tự nêu `master.key`).
- `marshalSealed(map) ([]byte, error)`: chép map; nếu có bí mật không rỗng → `LoadOrCreateKey`,
  `Seal` từng cái; `MarshalIndent`. `saveAll` dùng nó (phần ghi còn lại giữ nguyên).
- `updateServer`: `merged.Password = server.Password` cạnh `PrivateKey`.
- `mergeMetadata` (mapper.go): `server.Password = meta.Password`.
- `Repository.ListServers`: trả lỗi `loadAll` (`fmt.Errorf("load metadata: %w", err)`) thay vì
  warn + map rỗng.
- `Repository.GetPassword(alias)`: `loadAll` → `meta[alias].Password` (không parse config, không
  ghi file key). Thêm vào `ports.ServerRepository`.
- UI: `loadInitialData` và `refreshServerList` hiện lỗi `ListServers` bằng
  `showStatusTempColor(err.Error(), "#FF6B6B")`.
- Test (`metadata_manager_test.go`): `TestMetadata_SecretsEncryptedOnDisk`,
  `TestMetadata_RoundTripPasswordAndKey`, `TestMetadata_NoSecretsNoMasterKey`,
  `TestMetadata_LegacyPlaintextMigratedOnNextWrite` (qua `setPinned`),
  `TestMetadata_TamperedValueFailsLoadWithoutWrite`,
  `TestMetadata_MissingMasterKeyFailsWithoutWrite` (lỗi nêu `master.key`, byte file không đổi,
  không tạo key), `TestMetadata_ErrorsNeverContainSecret`, `TestRepo_GetPassword`,
  `TestRepo_RenameMovesPassword`.

### T3: Điểm móc cho sync

- `Export`: `meta, err := r.metadataManager.loadAll()`; lỗi → trả ra (push không upload gì);
  `b.Metadata, err = json.Marshal(meta)` (gọn, để `TestExport_ReadsConfigAndMetadata` giữ được
  phép so sánh byte; map nil → `{}` nhờ `make`).
- `Import`: kiểm version (có sẵn) → `json.Unmarshal(b.Metadata, &meta)` → có bí mật `IsSealed` →
  `errors.New("sync bundle contains encrypted values from another machine")` trước mọi thao tác
  ghi → `saveRawConfig` (có sẵn) → `marshalSealed(meta)` → tmp + rename có sẵn.
- Test: `TestExport_DecryptsSecrets`, `TestExport_FailsOnUndecryptable`,
  `TestImport_SealsWithLocalKey`, `TestImport_CreatesMasterKeyIfMissing`,
  `TestImport_RejectsSealedValuesBeforeWrite`, `TestExportImport_DifferentMasterKeys`. Test có sẵn
  `TestImport_WritesBothAndBacksUpConfig` so sánh file metadata từng byte với `{"new":{}}`; Import
  nay ghi qua `marshalSealed` có thụt lề, nên đổi assertion đó sang so sánh JSON đã decode (thay
  đổi duy nhất ở test có sẵn).

### T4: Khởi chạy ssh với askpass (kết nối, forward, SFTP)

- `services/askpass.go`: `var executable = os.Executable`;
  `passwordAuth(lookup func(alias string) (string, error), alias string) (args, env []string, err error)`:
  `lookup` lỗi → `err`; rỗng → `nil, nil, nil`; ngược lại
  `args = ["-o","PreferredAuthentications=password,keyboard-interactive","-o","NumberOfPasswordPrompts=1"]`,
  `env = append(os.Environ(), "SSH_ASKPASS="+exe, "SSH_ASKPASS_REQUIRE=force", "LAZYSSH_ASKPASS_ALIAS="+alias)`.
- `sshCmd(alias string, pre, post []string) (*exec.Cmd, error)` trong `server_service.go`, dùng cho
  `SSH` (`ssh <args> <alias>`), `SSHWithArgs` (`<extra> <args> <alias>`), `StartForward`
  (`<extra> <args> -N <alias>`); `cmd.Env = env` (nil khi không có mật khẩu). Lỗi từ
  `passwordAuth` → trả ra, không tạo tiến trình. Test kiểm `cmd.Args` / `cmd.Env` mà không chạy ssh.
- `validateServer`: `strings.ContainsAny(srv.Password, "\r\n")` → `password must be a single line`.
- `NewSFTPService(logger, password func(alias string) (string, error))`;
  `newSFTPServiceWithCmd(logger, password, cmdFor)` với `cmdFor func(alias string, authArgs
  []string) *exec.Cmd` (mặc định dựng `ssh <authArgs> -s -- <alias> sftp`); `Connect` gọi
  `passwordAuth(s.password, alias)` (lỗi → trả ra), dựng cmd, đặt `cmd.Env = env`.
  `cmd/main.go` truyền `serverRepo.GetPassword`. Các test sftp có sẵn truyền lookup trả `"", nil`
  và `helperCmd` bỏ qua `authArgs`.
- Test: `TestPasswordAuth_NoPasswordUnchanged` (Env nil, Args như hiện nay),
  `TestPasswordAuth_SetsAskpassEnvAndOptions`, `TestPasswordAuth_SecretNotInArgsOrEnv`,
  `TestPasswordAuth_LookupErrorAborts`, `TestSSHCmd_OptionsBeforeDestination` (kết nối, kèm tham
  số, forward), `TestSFTPCmd_PasswordServer`, `TestValidateServer_PasswordNewlineRejected`.

### T5: Chế độ helper askpass

- `services.Askpass(lookup func(string) (string, error), userHost func(string) (string, string,
  error), alias, prompt string, out io.Writer, tty func() (io.ReadWriteCloser, error)) int`:
  1. `p := strings.ToLower(strings.TrimRight(prompt, " \t"))`.
  2. Nếu `strings.HasSuffix(p, "password:")` và `userHost` thành công và `p` chứa
     `strings.ToLower(user+"@"+host)`: `pw, err := lookup(alias)`; `err` hoặc `pw == ""` → ghi lý do
     (không có bí mật) ra stderr, trả 1; ngược lại `fmt.Fprintln(out, pw)`, trả 0.
  3. Ngược lại chuyển tiếp: `tty()` lỗi → trả 1; ghi prompt vào tty, đọc một dòng
     (`bufio.Reader.ReadString('\n')`, cắt `\r\n`), `fmt.Fprintln(out, line)`, trả 0.
  `// ponytail: relayed input echoes (no x/term); fine for host-key yes/no and OTP codes.`
- `ResolveUserHost(alias)` (exported, trong `server_service.go`): parse đầu ra `ssh -G` lấy
  `user`, `hostname`, `hostkeyalias` (bỏ qua khi rỗng hoặc `none`); `resolveSSHDestination` chuyển
  sang dùng chung parser nhỏ `parseSSHG(out string) map[string]string`.
- `cmd/main.go`, những dòng đầu của `main()`:
  `if alias := os.Getenv("LAZYSSH_ASKPASS_ALIAS"); alias != "" { os.Exit(runAskpass(alias)) }`;
  `runAskpass` dựng `ssh_config_file.NewRepository(zap.NewNop().Sugar(), ...)`, lấy `os.Args[1]`
  nếu có, mở `/dev/tty` bằng `os.OpenFile(..., os.O_RDWR, 0)`.
- Test (fake, không chạy ssh thật): `TestAskpass_PrintsPasswordForTargetPrompt`,
  `TestAskpass_KeyboardInteractivePromptAnswered` (`(u@h) Password:`),
  `TestAskpass_OtherHostPromptNotAnswered` (được chuyển tiếp, không in mật khẩu),
  `TestAskpass_HostKeyPromptRelayed`, `TestAskpass_NoTTYExitsNonZero`,
  `TestAskpass_UnknownAliasExitsNonZero`, `TestAskpass_NoPasswordExitsNonZero`,
  `TestAskpass_LookupErrorExitsNonZeroWithoutSecret`, `TestAskpass_UserHostErrorRelays`,
  `TestParseSSHG_UserHostKeyAlias`.

### T6: Form, details pane, status

- `auth_method.go`: `const authIdentityFile, authEmbeddedKey, authPassword`;
  `authMethodOf(domain.Server) string`; `applyAuthMethod(*ServerFormData) error` (data-model.md);
  `authFieldsEnabled(method string) (key, password bool)` — hàm thuần để logic bật/tắt trường có test.
- `createAuthenticationForm`: item đầu tiên là dropdown `Auth method:` (chỉ số từ
  `authMethodOf(original)`), rồi các item embedded key có sẵn, rồi `Password:`
  (`tview.NewInputField().SetLabel("Password:").SetMaskCharacter('*')`, mặc định
  `original.Password`); `SetSelectedFunc` bật/tắt `SetDisabled` trên ba trường theo `authFieldsEnabled`.
- `ServerFormData` + `AuthMethod`, `Password`; `getFormData` đọc chúng; `getDefaultValues` điền
  chúng; `handleSave` gọi `applyAuthMethod` trước `resolvePrivateKey` (lỗi → cùng modal như lỗi
  import); `dataToServer` chép `Password`; `hasUnsavedChanges` so sánh `Password`.
- `server_details.go`: `if server.Password != "" { serverKey += " (password)" }`.
- `field_help.go`: mục `AuthMethod`, `Password` (không có ví dụ trông giống bí mật).
- Test: `TestApplyAuthMethod_PasswordClearsKey`, `TestApplyAuthMethod_PasswordRequired`,
  `TestApplyAuthMethod_IdentityFileClearsBoth`, `TestApplyAuthMethod_EmbeddedKeyClearsPassword`,
  `TestAuthMethodOf`, `TestAuthFieldsEnabled`, `TestServerDetails_PasswordNeverRendered`.

### T7: Tài liệu và rà soát bí mật

- README: xác thực mật khẩu (ghi chú OpenSSH ≥ 8.4), `master.key` (hãy sao lưu; không được sync;
  mất nó nghĩa là phải nhập lại bí mật hoặc pull từ sync).
- Rà soát bằng `grep -rn`: không lời gọi `zap` nào log giá trị `domain.Server`/`ServerMetadata`
  hay mật khẩu; `go test -race ./...` và `make build` đều xanh.

## Theo dõi độ phức tạp (Complexity Tracking)

Không có vi phạm.
