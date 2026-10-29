# Nghiên cứu: Xác thực bằng mật khẩu và mã hoá bí mật khi lưu trữ

Sau `/speckit-clarify` không còn `NEEDS CLARIFICATION`; các mục dưới ghi lại quyết định thiết kế
đưa ra khi lập kế hoạch, mỗi mục đối chiếu với mã hiện có.

## R1: Mã crypto khi lưu trữ đặt ở đâu (phân lớp)

- **Quyết định**: một package lá mới chỉ dùng stdlib `internal/secretbox` (cạnh `internal/logger`
  hiện có), chỉ được data adapter `internal/adapters/data/ssh_config_file` import. Adapter mã hoá
  khi ghi và giải mã khi đọc, nên `domain.Server` luôn giữ plaintext trong bộ nhớ và không có mã
  port, service hay UI nào biết đến việc mã hoá.
- **Lý do**: Hiến pháp I cấm data adapter import `services` (nơi có `sync_crypto.go`). Một port
  (`ports.SecretCipher`) hiện thực trong `services` rồi inject vào adapter sẽ là interface chỉ có
  một hiện thực và không cần fake (bị nguyên tắc V loại). Package lá không import gì trong dự án
  nên không thể tạo vòng phụ thuộc giữa các lớp, và đã có tiền lệ là `internal/logger`.
- **Phương án khác**: đặt crypto trong `domain` (domain không nên làm file I/O cho `master.key`);
  chuyển `sync_crypto.go` vào package mới (xáo trộn mã 002 đã xong chỉ vì ~10 dòng dùng chung; KDF
  khác nhau, phần chung duy nhất là `aes.NewCipher` + `cipher.NewGCM`).

## R2: Định dạng giá trị và file key

- **Quyết định**: `enc:v1:` + `base64.StdEncoding(nonce[12] || gcm.Seal(plaintext))`, AES-256-GCM,
  không có additional data. `master.key` = 32 byte ngẫu nhiên thô, tạo bằng
  `os.OpenFile(O_WRONLY|O_CREATE|O_EXCL, 0o600)` sau `MkdirAll(dir, 0o700)`. Đúng 32 byte, nếu
  không là "hỏng". Key được nạp lười: chỉ khi một giá trị lưu có tiền tố (đọc) hoặc khi phải ghi
  một bí mật không rỗng (ghi; tạo nếu thiếu).
- **Lý do**: Giống primitive của `sync_crypto.go` (`nonceSize = 12`, `keySize = 32`, GCM).
  `O_EXCL` bảo đảm key đã có (kể cả hỏng) không bao giờ bị ghi đè (FR-007). Nạp lười nghĩa là người
  dùng không có bí mật sẽ không bao giờ có `master.key`.
- **Phương án khác**: gắn alias làm additional data của GCM (sẽ hỏng khi đổi tên alias mà không
  được lợi gì: kẻ tráo được giá trị giữa các alias trong file thì vốn đã đọc được `master.key`);
  file key dạng hex/base64 (không ai ngoài lazyssh đọc nó).

## R3: Chính sách khi đọc thất bại

- **Quyết định**: bất kỳ giá trị nào không giải mã được làm `metadataManager.loadAll` thất bại với
  `decrypt <field> of "<alias>" in <metadata path>: <cause>`; key thiếu/hỏng thất bại với lỗi nêu
  `<dir>/master.key`. Mọi hàm ghi hiện có (`updateServer`, `deleteServer`, `setPinned`,
  `recordSSH`) đã trả về sớm khi `loadAll` lỗi, nên không gì bị ghi đè. `Repository.ListServers`
  hiện nuốt lỗi `loadAll` (warn + metadata rỗng); nó PHẢI trả lỗi ra, và lần nạp lúc khởi động cùng
  `refreshServerList` của UI hiển thị lỗi màu đỏ trên status bar (FR-009).
- **Lý do**: Thất bại đóng (fail closed) là lựa chọn duy nhất không thể làm mất bí mật đã mã hoá.
  Khôi phục: trả lại `master.key`, hoặc Pull từ sync (Import thay toàn bộ metadata và có thể tạo
  key mới, FR-016).
- **Phương án khác**: bỏ các giá trị không giải mã được rồi chạy tiếp (lần lưu sau sẽ xoá chúng:
  mất dữ liệu).

## R4: Chuyển đổi plaintext cũ

- **Quyết định**: `secretbox.Open` trả nguyên giá trị không có tiền tố `enc:`; `saveAll` luôn mã
  hoá bí mật không rỗng. Mỗi lần ghi đều ghi lại mọi mục, nên lần ghi đầu tiên dưới bất kỳ hình thức
  nào (bộ đếm kết nối, pin, sửa) chuyển đổi toàn bộ file.
- **Lý do**: Không cần bước migration, không cần cờ phiên bản trong `metadata.json`.

## R5: Đưa mật khẩu cho OpenSSH

- **Quyết định**: self-askpass. Với server có mật khẩu, tiến trình ssh con nhận
  `cmd.Env = append(os.Environ(), "SSH_ASKPASS="+exe, "SSH_ASKPASS_REQUIRE=force",
  "LAZYSSH_ASKPASS_ALIAS="+alias)` và các tham số
  `-o PreferredAuthentications=password,keyboard-interactive -o NumberOfPasswordPrompts=1` chèn
  trước đích. `exe` lấy từ `os.Executable()` (biến package trong `services` để test thay được).
  Không đụng tới `DISPLAY`.
- **Lý do**: `read_passphrase()` của OpenSSH dùng chương trình askpass cho mọi prompt khi
  `SSH_ASKPASS_REQUIRE=force` (OpenSSH ≥ 8.4, năm 2020), không phụ thuộc `DISPLAY` hay có TTY hay
  không, nên bao phủ cả đường SFTP (stdin là pipe) và forward chạy nền. Client cũ bỏ qua biến này
  và hỏi trên TTY: lùi về nhẹ nhàng. Máy hiện tại: OpenSSH 10.6. `NumberOfPasswordPrompts=1` ngăn
  ssh gửi lại mật khẩu sai ba lần. `PreferredAuthentications` bỏ qua các lần thử publickey (không
  bị "Too many authentication failures" khi agent có nhiều key, không có prompt passphrase của key
  tới helper).
- **Phương án khác**: `sshpass` (thêm binary, mật khẩu trong argv/env hoặc xoay xở fd); SSH client
  nội bộ bằng `golang.org/x/crypto/ssh` (viết lại kết nối + SFTP, mất ngữ nghĩa ssh config của
  người dùng); đặt `DISPLAY` cho client trước 8.4 (còn đòi không có TTY điều khiển, tức `setsid`,
  làm hỏng phiên tương tác).

## R6: Prompt nào được nhận mật khẩu

- **Quyết định**: helper chỉ trả lời bằng mật khẩu đã lưu khi prompt, sau khi chuyển thường và cắt
  khoảng trắng bên phải, kết thúc bằng `password:` và chứa `<user>@<host>` với `user` và `host`
  (`hostkeyalias` nếu có, nếu không thì `hostname`) lấy từ `ssh -G <alias>`. Mọi prompt khác được
  chuyển tiếp: mở `/dev/tty` đọc-ghi, ghi prompt, đọc một dòng, in ra stdout, thoát 0; nếu không mở
  được `/dev/tty` thì thoát 1.
- **Lý do**: OpenSSH dựng prompt mật khẩu là `"%s@%s's password: "` (user, host key alias hoặc
  host) và, ở các bản hiện nay, thêm tiền tố `"(user@host) "` cho prompt keyboard-interactive.
  Tiến trình ssh con của `ProxyJump` kế thừa môi trường, nên nếu không kiểm `user@host` thì host
  jump sẽ nhận mật khẩu của server đích. Prompt xác nhận host key và OTP vẫn phải tới được người
  dùng; chính ssh cũng nói chuyện với `/dev/tty`, nên helper làm y như vậy (TUI được suspend trong
  cả lúc kết nối lẫn lúc kết nối SFTP, `handlers.go` / `sftp_view.go`). Forward chạy nền dùng
  `Setsid` (không có TTY điều khiển) nên chuyển tiếp thất bại gọn gàng.
- **Giới hạn**: đầu vào được chuyển tiếp sẽ bị echo (không dùng `golang.org/x/term`, hiến pháp
  III). Chấp nhận được với `yes`/`no` của host key và mã một lần; passphrase của key không bao giờ
  tới helper (R5). OpenSSH cũ không có tiền tố `(user@host)` cho keyboard-interactive: prompt được
  chuyển tiếp và người dùng tự gõ mật khẩu. Đánh dấu `ponytail:` trong mã.
- **Phương án khác**: trả lời mọi prompt mật khẩu (lộ thông tin đăng nhập cho host jump); thoát mã
  khác 0 với prompt không phải mật khẩu (làm hỏng lần kết nối đầu tới host chưa biết).

## R7: Helper và các đường kết nối tra mật khẩu thế nào

- **Quyết định**: thêm một method vào `ports.ServerRepository`: `GetPassword(alias string)
  (string, error)` (chuỗi rỗng khi không có). Adapter hiện thực bằng `metadataManager.loadAll()`
  thôi (không parse ssh config, không ghi file key, không ghi gì: FR-015). `serverService.SSH`,
  `SSHWithArgs`, `StartForward` và `sftpService.Connect` gọi nó qua hàm dùng chung
  `passwordAuth(lookup, alias) (args, env []string, err error)` trong `services/askpass.go`.
  `NewSFTPService` nhận thêm một hàm tra cứu `func(alias) (string, error)`
  (`serverRepo.GetPassword`), nên test của nó không cần fake repository.
- **Lý do**: Dùng lại port có sẵn; hiện không có test fake nào hiện thực `ServerRepository`, nên
  đổi interface không tốn gì. Lỗi tra cứu (ví dụ key hỏng) làm kết nối thất bại với lỗi nêu tên
  file thay vì âm thầm kết nối không có mật khẩu.

## R8: Điểm vào askpass

- **Quyết định**: việc đầu tiên trong `main()`, trước khi tạo logger:
  `if alias := os.Getenv("LAZYSSH_ASKPASS_ALIAS"); alias != "" { os.Exit(runAskpass(alias)) }`.
  `runAskpass` dựng repository với logger rỗng (`zap.NewNop().Sugar()`), và gọi
  `services.Askpass(repo.GetPassword, services.ResolveUserHost, alias, prompt, os.Stdout, openTTY)`,
  hàm chứa toàn bộ logic và được unit test bằng fake. `ResolveUserHost` parse đầu ra `ssh -G` (dùng
  chung parser với `resolveSSHDestination` hiện có). Mã thoát 0 khi trả lời, 1 trong mọi trường hợp
  khác.
- **Lý do**: `SSH_ASKPASS` được thực thi với đúng một tham số (prompt), nên không truyền được cờ
  kiểu `--askpass`; biến môi trường là tín hiệu duy nhất. Bỏ qua logger giúp helper không ghi gì
  (FR-015).

## R9: Điểm móc cho sync

- **Quyết định**: `Repository.Export` dựng `b.Metadata` từ `metadataManager.loadAll()` (đã giải
  mã) marshal bằng `json.Marshal` (gọn) thay vì byte thô của file; map rỗng vẫn ra `{}`.
  `Repository.Import` unmarshal `b.Metadata` vào `map[string]ServerMetadata`, từ chối mọi giá trị
  có tiền tố `enc:`, rồi ghi qua cùng hàm marshal có mã hoá với `saveAll` vào đường temp-file +
  rename hiện có (tạo `master.key` nếu thiếu).
- **Lý do**: Cả hai đầu nằm trong adapter sở hữu việc mã hoá; `SyncService`, kiểu bundle và version
  không đổi (FR-016). Các trường JSON lạ trong `metadata.json` vốn đã bị mỗi lần ghi
  `updateServer` bỏ đi, nên marshal lại trong Export không thay đổi gì đáng kể.

## R10: Bộ chọn phương thức trên form

- **Quyết định**: dropdown `Auth method:` ở đầu tab Authentication. `SetSelectedFunc` của nó gọi
  `SetDisabled` trên `PrivateKey:`, `Import key from file:` và `Password:` (tview `InputField`/
  `TextArea` hỗ trợ). `Password:` là `tview.NewInputField()` với `SetMaskCharacter('*')`.
  `ServerFormData` thêm `AuthMethod` và `Password`; hàm thuần `applyAuthMethod(*ServerFormData)
  error` xoá bí mật không được chọn và bắt buộc `password is required`; `validateServer` trong
  `services` từ chối `\n`/`\r` (`password must be a single line`) để mọi đường lưu đều được bao phủ.
- **Lý do**: Vô hiệu hoá giữ nguyên bố cục form và các lần tra theo nhãn trong `getFormData` (xoá
  item sẽ làm lệch chỉ số và mất giá trị đã gõ khi chuyển qua lại).
