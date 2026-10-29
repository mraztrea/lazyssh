# Research: Thư mục làm việc mặc định (Home Dir) cho mỗi profile SSH

Không có mục NEEDS CLARIFICATION trong Technical Context; các quyết định dưới đây chốt cách hiện thực.

## R1: Cách bắt ssh "cd" rồi mở shell

- **Decision**: chạy `ssh -o RemoteCommand=none -t … <alias> 'cd <dir>; exec "$SHELL" -l'`.
- **Rationale**: ssh không có tuỳ chọn "thư mục làm việc"; cách chuẩn là truyền lệnh remote và yêu cầu
  TTY (`-t`) để shell tương tác. sshd luôn đặt `SHELL` cho phiên nên `exec "$SHELL" -l` mở đúng login
  shell của tài khoản. Dấu `;` thay vì `&&` để `cd` lỗi vẫn vào shell (FR-005). `-l` để shell đọc
  profile như một phiên `ssh host` bình thường.
- **Alternatives considered**: `ssh host -t 'cd x && bash'` (cứng shell, bỏ profile); `RequestTTY yes`
  + `RemoteCommand` ghi vào ssh config (đụng `~/.ssh/config`, trái FR-002); gửi `cd` qua stdin sau khi
  kết nối (không có cách sạch với tty).

## R2: Xung đột với `RemoteCommand` trong ssh config

- **Decision**: luôn thêm `-o RemoteCommand=none` trên dòng lệnh khi có Home Dir, và tự ghép
  `RemoteCommand` của profile (nếu có, khác `none`) vào sau `cd`: `cd <dir>; <RemoteCommand>`.
- **Rationale**: OpenSSH báo `Cannot execute command-line and remote command.` khi config có
  `RemoteCommand` mà dòng lệnh cũng có lệnh. Tuỳ chọn `-o` trên dòng lệnh thắng config, và `none` xoá
  `RemoteCommand` (OpenSSH 7.6+, đã được code `BuildSSHCommand` dựa vào). Ghép lại lệnh của người dùng
  giữ đúng ý họ (tmux/screen) nhưng từ Home Dir (Clarification 1).
- **Alternatives considered**: bỏ qua Home Dir khi có RemoteCommand (mất tính năng với người dùng tmux);
  không thêm `RemoteCommand=none` (lỗi fatal với profile có RemoteCommand).
- **Cần kiểm thủ công** (quickstart bước 6): profile có `RemoteCommand` trong `~/.ssh/config`.

## R3: Quoting và `~`

- **Decision**: single-quote toàn bộ đường dẫn (`'` → `'\''`); riêng `~` và tiền tố `~/` thay bằng
  `"$HOME"` không quote để shell expand.
- **Rationale**: single-quote vô hiệu mọi ký tự đặc biệt (`$`, `;`, khoảng trắng) nên giá trị không bao
  giờ chạy như lệnh (FR-010). `"$HOME"` hoạt động trên sh/bash/zsh/fish, còn `~` trong dấu nháy thì
  không expand.
- **Alternatives considered**: `${SHELL:-sh}`/`${HOME}` (fish không hiểu `${}`); để nguyên không quote
  (lỗi với khoảng trắng).

## R4: SFTP

- **Decision**: resolve ở client: `Getwd()` của `pkg/sftp` trả thư mục home của tài khoản; `~`/đường dẫn
  tương đối ghép với nó bằng `path.Join`; `Load` lỗi thì về `wd` và báo `Home dir not found`.
- **Rationale**: SFTP không có khái niệm `cd` phía server, pane chỉ cần một đường dẫn tuyệt đối; giữ
  nguyên luồng khởi tạo hiện có (FR-007, FR-008).
- **Alternatives considered**: `client.RealPath(dir)` trước khi Load (thêm một round-trip mà `Load` đã
  báo lỗi đủ rõ).

## R5: Đưa Home Dir tới service SSH

- **Decision**: đổi `ServerService.SSH(alias)` → `SSH(server domain.Server)` (và `SSHWithArgs`); UI
  đã có `domain.Server` được chọn.
- **Rationale**: service cần cả `HomeDir` và `RemoteCommand`; UI đã có đủ, tránh thêm method
  repository hay đọc lại config. Không có mock nào hiện thực `ServerService` trong test nên chỉ đổi 2
  caller ở `handlers.go`.
- **Alternatives considered**: thêm `ServerRepository.GetServer(alias)` (method mới cho một chỗ dùng);
  `ListServers("")` rồi tìm alias (đọc lại toàn bộ config mỗi lần SSH).
