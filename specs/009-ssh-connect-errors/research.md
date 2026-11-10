# Research: Báo lỗi kết nối SSH/SFTP và xử lý host key đã đổi

Không có mục NEEDS CLARIFICATION trong Technical Context. Các quyết định dưới đây được kiểm bằng
OpenSSH_10.6p1 và một `sshd` tạm chạy không cần root trên cổng cao (`sshd -f <cfg> -d -e`), với
`known_hosts` và config trong thư mục tạm.

## R1: Phân biệt "kết nối lỗi" với "phiên kết thúc"

- **Decision**: chỉ mã thoát 255 của `ssh` là lỗi kết nối. Mã khác (mã của shell/lệnh remote) và
  tiến trình bị tín hiệu (Ctrl-C ở dấu nhắc mật khẩu → `ExitCode() == -1`) là thoát bình thường.
- **Rationale**: `ssh(1)`: "exits with the exit status of the remote command or with 255 if an error
  occurred". Đã chạy thật: phiên `exit 3` → mã 3; mọi lỗi ở bảng R3 → 255.
- **Alternatives considered**: hiện hộp thoại cho mọi mã khác 0 (báo động giả sau mỗi phiên có lệnh
  cuối lỗi); phân loại chỉ bằng stderr không xét mã (phiên dài in thông báo cảnh báo vẫn bị coi là
  lỗi).
- **Giới hạn chấp nhận**: lệnh remote tự `exit 255` cũng bị coi là lỗi (hộp thoại "không nhận diện
  được" với nguyên văn đầu ra).

## R2: Giữ stderr của ssh mà không làm hỏng terminal

- **Decision**: `cmd.Stderr = io.MultiWriter(os.Stderr, tail)` với `tail` là `tailBuffer` sẵn có
  (giữ 4 KiB cuối, đang dùng cho SFTP); `cmd.WaitDelay = time.Second`; coi `exec.ErrWaitDelay` như
  thành công.
- **Rationale**: OpenSSH hỏi mật khẩu, passphrase và xác nhận host key qua `/dev/tty`, không qua
  stderr; stdin/stdout vẫn là terminal nên ssh vẫn cấp TTY và raw mode như cũ. Đã chạy thật qua một
  chương trình Go dùng đúng cách nối này trong pty: câu hỏi "Are you sure you want to continue
  connecting (yes/no/[fingerprint])?" kèm fingerprint, câu hỏi passphrase, shell tương tác và mã
  thoát remote đều đúng; phần đuôi stderr thu được đầy đủ banner host key. Askpass (mật khẩu đã lưu)
  không đổi: helper vẫn mở tty riêng, stderr của nó đi vào cùng pipe.
- **WaitDelay**: khi stderr là pipe, `Wait` chờ mọi tiến trình giữ đầu ghi đóng lại. Master
  `ControlPersist` của OpenSSH tự chuyển stdio sang `/dev/null` khi chạy nền, nhưng `ProxyCommand`
  hoặc master chạy với debug có thể giữ pipe; `WaitDelay` (chỉ tính sau khi ssh đã thoát) chặn việc
  TUI bị treo. Không giết phiên đang chạy.
- **Lưu ý**: ở raw mode ssh in `\r\n`; bộ phân loại bỏ `\r`.
- **Alternatives considered**: `ssh -E logfile` (đổi hành vi log, thông báo không còn trên terminal);
  pty riêng (phức tạp, cần dependency); in lỗi rồi chờ phím Enter trước khi về TUI (không phân loại,
  không có hành động xử lý).

## R3: Mẫu thông báo OpenSSH (thu từ chạy thật)

| Tình huống | Thông báo (rút gọn) | Nguyên nhân hiển thị |
|---|---|---|
| Host key đổi | `@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @` … `The fingerprint for the ED25519 key sent by the remote host is` / `SHA256:….` / `Offending ED25519 key in /tmp/x/known_hosts:1` / `Host key for [127.0.0.1]:22422 has changed and you have requested strict checking.` / `Host key verification failed.` | Host key changed |
| Host lạ + strict / trả lời `no` | `No ED25519 host key is known for [127.0.0.1]:22422 and you have requested strict checking.` / `Host key verification failed.` | Host key not verified |
| Quyền key | `WARNING: UNPROTECTED PRIVATE KEY FILE!` / `Permissions 0644 for '/tmp/x/id' are too open.` / `Load key "/tmp/x/id": bad permissions` / `user@127.0.0.1: Permission denied (publickey,keyboard-interactive).` | Private key permissions too open |
| Sai xác thực | `user@127.0.0.1: Permission denied (publickey,keyboard-interactive).`; `Received disconnect from … Too many authentication failures` | Authentication failed |
| Cổng đóng | `ssh: connect to host 127.0.0.1 port 22499: Connection refused` | Connection refused |
| Hết giờ | `ssh: connect to host 10.255.255.1 port 22422: Connection timed out` | Connection timed out |
| DNS | `ssh: Could not resolve hostname nonexistent.invalid: Name or service not known` | Host name not found |
| Không có route | `ssh: connect to host … port 22: No route to host` / `Network is unreachable` (cùng định dạng `strerror`, không tái tạo được trên máy thử) | Host unreachable |
| Khác | ví dụ `command-line: line 0: Bad configuration option: foo` | SSH connection failed |

- **Thứ tự so khớp**: host key đổi → host key chưa xác minh → quyền key (đứng trước vì luôn kèm
  "Permission denied") → xác thực → DNS → refused → timeout → unreachable → khác.
- **Tên host**: lấy từ `Host key for <name> has changed` (khớp cả dạng cũ `ED25519 host key for …`):
  cổng khác 22 ra đúng `[host]:port`, có `HostKeyAlias` thì ra alias khoá — đúng tên
  `ssh-keygen -R` cần. Không đoán từ alias lazyssh.
- **File**: lấy từ dòng `Offending <TYPE> key in <file>:<line>` đầu tiên (bỏ qua dạng
  `Offending key for IP in …`). Thiếu tên host hoặc file → không đề nghị xoá, chỉ cảnh báo.

## R4: Xoá key cũ

- **Decision**: `ssh-keygen -R <host> -f <file>` (argv trực tiếp, không qua shell), sau đó chạy lại
  đúng thao tác ban đầu; ssh tự hỏi xác nhận fingerprint mới (StrictHostKeyChecking mặc định `ask`).
- **Rationale**: công cụ chuẩn, hiểu `known_hosts` dạng hash, tự lưu `<file>.old`. Đã chạy thật:
  `# Host [127.0.0.1]:22422 found: line 1` / `known_hosts updated.` / `Original contents retained as
  known_hosts.old`, rồi lần kết nối sau hỏi `The authenticity of host '[127.0.0.1]:22422' can't be
  established. ED25519 key fingerprint is: SHA256:…`. `getopt` lấy phần tử argv kế tiếp làm đối số
  của `-R`/`-f` kể cả khi bắt đầu bằng `-`, nên không có chèn tuỳ chọn.
- **Alternatives considered**: tự sửa file `known_hosts` (phải xử lý hash, marker, nhiều file);
  `StrictHostKeyChecking=accept-new`/`no` (cấm — tự chấp nhận key); xoá theo số dòng (dòng có thể
  chứa nhiều host).

## R5: Áp dụng cho SFTP

- **Decision**: trong phạm vi. `sftpService.Connect` đã giữ stderr vào `tailBuffer`; khi bắt tay
  SFTP thất bại, trả về lỗi đã phân loại thay vì chỉ dòng cuối. UI dùng chung hộp thoại; Retry/xoá
  key mở lại file manager.
- **Rationale**: chi phí vài dòng. Câu hỏi xác nhận host key khi kết nối lại vẫn đi qua `/dev/tty`
  trong lúc TUI bị suspend như hiện tại.

## R6: Hiển thị

- **Decision**: dùng `tview.NewModal()` như hộp thoại Delete/Save hiện có; mọi văn bản động đi qua
  `tview.Escape` (ví dụ `[127.0.0.1]:22422` sẽ bị tview hiểu nhầm là thẻ màu); chỉ hiện 8 dòng cuối
  của stderr.
- **Alternatives considered**: status bar (2 giây, một dòng — đúng là vấn đề hiện tại); trang riêng
  có cuộn (thêm thành phần mới không cần thiết).
