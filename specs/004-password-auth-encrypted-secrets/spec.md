# Đặc tả tính năng: Xác thực bằng mật khẩu và mã hoá bí mật khi lưu trữ

**Feature Branch**: `004-password-auth-encrypted-secrets`

**Ngày tạo**: 2026-10-29

**Trạng thái**: Draft

**Input**: Mô tả của người dùng: "Hiện tại server có thể đăng nhập bằng một file private key đã chọn. Bổ sung: (1) cho phép nhập mật khẩu thay cho private key; (2) với private key có hai lựa chọn: chọn đường dẫn key, hoặc import file key (import thì lưu nội dung key vào config để đồng bộ lên gist); (3) mật khẩu và private key được lưu phải được mã hoá trong file config."

**Xây dựng trên**: `specs/001-embedded-private-keys` (key nhúng, `Import key from file:`),
`specs/002-encrypted-sync` (sync bundle), `specs/003-sftp-file-manager` (phiên SFTP qua `ssh -s`).

**Đã có sẵn (không đặc tả lại)**: mục (2) của yêu cầu. *Đường dẫn* key chính là trường `Keys:`
(`IdentityFile`) hiện có; *import* file key là trường `Import key from file:` của spec 001, lưu
nội dung key vào `~/.lazyssh/metadata.json` (`private_key`) nên nó đi kèm sync bundle. Tính năng
này chỉ làm cho việc chọn giữa các phương thức hiện rõ trên form (FR-001).

## Clarifications

### Session 2026-10-29

Trả lời không tương tác (answered non-interactively) dựa trên các quyết định đi kèm yêu cầu tính năng (phạm vi, lưu trữ, askpass, mã hoá khi lưu, sync, ngoài phạm vi, quy tắc bí mật, kiểm thử).

- Q: Phương thức xác thực được chọn có được lưu thành một trường riêng không, và bí mật của phương thức kia xử lý thế nào khi lưu? → A: Không lưu; phương thức được suy ra từ bí mật nào đang có (password, nếu không thì embedded key, nếu không thì identity file). Dropdown chỉ là công cụ hiển thị; khi lưu, nó xoá bí mật của các phương thức không được chọn để một server không bao giờ mang cả password lẫn embedded key (quyết định 2; FR-001, FR-003).
- Q: lazyssh chuyển mật khẩu đã lưu cho ssh hệ thống thế nào mà không dùng `sshpass` hay SSH client nội bộ? → A: lazyssh tự chạy lại chính nó làm chương trình `SSH_ASKPASS` với `SSH_ASKPASS_REQUIRE=force` và `LAZYSSH_ASKPASS_ALIAS=<alias>`; helper tra mật khẩu theo alias rồi in ra. Không đặt `DISPLAY`; cần OpenSSH ≥ 8.4 để không bị hỏi mật khẩu, client cũ hơn chỉ đơn giản hỏi trên terminal. Mật khẩu không bao giờ nằm trong argv hay env. Áp dụng cho kết nối SSH, port forward và đường SFTP `ssh -s` (quyết định 3; FR-011–FR-015).
- Q: Helper askpass làm gì với các prompt không phải prompt mật khẩu, và mật khẩu sai được thử lại bao nhiêu lần? → A: Chỉ prompt kết thúc bằng `password:` và chứa đúng `user@host` của server đích (lấy từ `ssh -G`) mới nhận mật khẩu đã lưu, nên host `ProxyJump` không bao giờ nhận được nó; mọi prompt khác (xác nhận host key, mã một lần) được chuyển tiếp tới `/dev/tty`, hoặc thất bại nếu không có terminal. lazyssh thêm `-o PreferredAuthentications=password,keyboard-interactive -o NumberOfPasswordPrompts=1`, nên mật khẩu sai chỉ được thử một lần (quyết định 3 "ưu tiên PreferredAuthentications=password"; FR-011, FR-013, FR-014).
- Q: Mật khẩu và embedded key được mã hoá trong `metadata.json` thế nào, và giá trị plaintext cũ xử lý ra sao? → A: AES-256-GCM với key ngẫu nhiên 32 byte riêng cho từng máy tại `~/.lazyssh/master.key` (0600, tạo khi cần lần đầu), nonce mới cho mỗi giá trị, lưu dạng `enc:v1:<base64>`; không thêm dependency. Giá trị không có tiền tố được đọc như plaintext cũ và được mã hoá ở lần ghi kế tiếp. `master.key` thiếu/hỏng hoặc giá trị bị sửa chỉ khoá server có giá trị đó: server vẫn hiện với lỗi nêu tên file (không bao giờ nêu bí mật), mật khẩu không dùng được, giá trị mã hoá được giữ nguyên trên đĩa qua mọi lần ghi cho tới khi người dùng nhập bí mật mới (quyết định 4, 5; FR-006–FR-009; người dùng chọn "chỉ chặn server lỗi" ngày 2026-10-29).
- Q: Sync bundle có thay đổi không, và `master.key` có được sync không? → A: Không đổi định dạng (bundle version 1). Bên trong bundle (vốn đã được mã hoá), `private_key` và `password` đi dưới dạng plaintext; khi pull chúng được mã hoá lại bằng `master.key` của máy cục bộ, file này không bao giờ được sync (quyết định 5, 6; FR-016, FR-017).

## Kịch bản người dùng & Kiểm thử (User Scenarios & Testing) *(bắt buộc)*

### User Story 1 - Đăng nhập server bằng mật khẩu đã lưu (Priority: P1)

Người dùng có server chỉ chấp nhận mật khẩu mở form server, chọn `Auth method: password` ở tab
Authentication, gõ mật khẩu vào trường `Password:` (bị che) rồi lưu. Từ đó, nhấn Enter trên server
này (cũng như mở trình quản lý file SFTP và bật port forward) sẽ đăng nhập mà không hỏi mật khẩu.
Mật khẩu không bao giờ hiện trên màn hình, trong details pane, status bar hay file log.

**Vì sao ưu tiên này**: Đây là khả năng mới người dùng yêu cầu; thiếu nó thì server chỉ dùng mật
khẩu vẫn phải gõ mật khẩu mỗi lần kết nối.

**Kiểm thử độc lập**: Với một sshd thử nghiệm (hoặc một `ssh` giả ghi lại môi trường và gọi chương
trình askpass như OpenSSH), lưu server có mật khẩu, kết nối, xác nhận đăng nhập thành công trong khi
mật khẩu không xuất hiện trong danh sách tham số lẫn môi trường của ssh.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** server lưu với phương thức `password` và mật khẩu đúng, **When** người dùng kết nối,
   **Then** phiên mở ra mà không hiện prompt mật khẩu nào.
2. **Given** cùng server đó, **When** người dùng mở trình quản lý file SFTP, **Then** hai pane nạp
   xong mà không hiện prompt mật khẩu.
3. **Given** server lưu mật khẩu sai, **When** người dùng kết nối, **Then** ssh thử mật khẩu đã lưu
   đúng một lần, thất bại với thông báo `Permission denied` quen thuộc và không thử lại.
4. **Given** server dùng mật khẩu chưa kết nối lần nào (host key chưa biết), **When** người dùng
   kết nối, **Then** câu hỏi xác nhận host key hiện trên terminal và câu trả lời của người dùng
   được dùng, y như khi không có tính năng này.
5. **Given** server có mật khẩu, **When** details pane hiển thị hoặc server được ghi log, **Then**
   chỉ có dấu `(password)` (không bao giờ là giá trị) được hiển thị.

---

### User Story 2 - Bí mật trong metadata.json được mã hoá (Priority: P1)

Người dùng có lưu embedded key hoặc mật khẩu mở `~/.lazyssh/metadata.json` bằng trình soạn thảo
và không thấy nội dung key hay mật khẩu nào, chỉ thấy các giá trị `enc:v1:` không đọc được. Một key
riêng cho từng máy ở `~/.lazyssh/master.key` (mode 0600) bảo vệ chúng. Mọi thứ khác trong lazyssh
hoạt động như cũ: key vẫn được ghi ra cho OpenSSH, server vẫn kết nối được.

**Vì sao ưu tiên này**: Người dùng đặt đây là yêu cầu bắt buộc ("phải được mã hoá"); lưu trường
mật khẩu mới dạng plaintext là không chấp nhận được, nên nó phát hành cùng Story 1.

**Kiểm thử độc lập**: Lưu server với key và mật khẩu sinh ngẫu nhiên vào một home tạm; đọc thô
`metadata.json` và xác nhận không bí mật nào xuất hiện và cả hai trường bắt đầu bằng `enc:v1:`;
nạp lại và xác nhận server mang đúng giá trị gốc.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** chưa có `master.key`, **When** server đầu tiên có key hoặc mật khẩu được lưu,
   **Then** `~/.lazyssh/master.key` được tạo với mode 0600 và bí mật được lưu đã mã hoá.
2. **Given** `metadata.json` do lazyssh bản cũ ghi với `private_key` plaintext, **When** lazyssh
   nạp nó, **Then** key được dùng như trước, và **When** lazyssh ghi `metadata.json` lần kế tiếp vì
   bất kỳ lý do gì (sửa, pin, kết nối), **Then** key được lưu đã mã hoá.
3. **Given** một giá trị mã hoá có ciphertext bị sửa, **When** lazyssh nạp metadata, **Then** nó
   báo lỗi nêu tên `metadata.json` và alias, không bao giờ nêu giá trị, và không ghi đè file.
4. **Given** có giá trị mã hoá nhưng `master.key` bị xoá hoặc hỏng, **When** lazyssh nạp metadata,
   **Then** nó báo lỗi nêu tên `~/.lazyssh/master.key`, không tạo key mới và không ghi đè
   `metadata.json`.

---

### User Story 3 - Chọn phương thức xác thực trên form (Priority: P2)

Người dùng mở tab Authentication thấy dropdown `Auth method:` với `identity file`, `embedded key`
và `password`. Chọn một mục thì chỉ các trường của phương thức đó được bật (các trường khác bị vô
hiệu hoá): `identity file` trỏ tới trường `Keys:` ở tab Basic, `embedded key` bật `PrivateKey:` và
`Import key from file:`, `password` bật `Password:`. Mở server có sẵn thì phương thức được chọn sẵn
theo dữ liệu đang lưu.

**Vì sao ưu tiên này**: Mô hình dữ liệu chạy được mà không cần nó (Story 1-2), nhưng chính nó làm
ba lựa chọn người dùng yêu cầu hiện rõ và ngăn một server âm thầm mang hai bí mật.

**Kiểm thử độc lập**: Mở form cho server có mật khẩu, có embedded key, và không có gì; xác nhận
phương thức chọn sẵn; đổi phương thức, lưu, nạp lại và xác nhận chỉ còn bí mật của phương thức
được chọn.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** server có mật khẩu đã lưu, **When** form mở, **Then** `Auth method:` là `password` và
   `Password:` hiện giá trị dưới dạng che.
2. **Given** server có embedded key, **When** người dùng đổi sang `password`, nhập mật khẩu và
   lưu, **Then** embedded key và file key của nó bị xoá, mật khẩu được lưu.
3. **Given** server có mật khẩu, **When** người dùng đổi sang `identity file` và lưu, **Then** mật
   khẩu bị xoá.

---

### User Story 4 - Sync mang theo mật khẩu và key giữa các máy (Priority: P2)

Người dùng push từ máy A và pull trên máy B (có `master.key` riêng, khác A). Mật khẩu và embedded
key tới B vẫn dùng được và được lưu mã hoá bằng key của B. Bản thân `master.key` không bao giờ được
sync.

**Vì sao ưu tiên này**: Người dùng muốn key import được "đồng bộ lên gist"; mã hoá khi lưu không
được làm hỏng sync hiện có, nhưng không thêm thao tác mới nào cho người dùng.

**Kiểm thử độc lập**: Trong hai home tạm với master key khác nhau, export từ A, import vào B, rồi
nạp B và xác nhận bí mật khớp và `metadata.json` của B chỉ chứa giá trị `enc:v1:`.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** server có mật khẩu trên A, **When** A push và B pull, **Then** B kết nối được bằng mật
   khẩu đó và `metadata.json` của B lưu nó đã mã hoá bằng key của B.
2. **Given** sync bundle do lazyssh bản cũ tạo (bên trong có `private_key` plaintext), **When** B
   pull nó, **Then** key được import và lưu đã mã hoá.

### Trường hợp biên (Edge Cases)

- **Mật khẩu có ký tự xuống dòng**: bị từ chối khi lưu với `password must be a single line` (giao
  thức askpass dựa trên dòng).
- **Server vừa có mật khẩu vừa có file key trong `Keys:`**: phương thức password thắng cho các kết
  nối do lazyssh khởi chạy (FR-011); `Keys:` giữ nguyên.
- **Askpass được gọi cho alias không có mật khẩu, alias không tồn tại, hoặc metadata không giải mã
  được**: helper không in gì và thoát với mã khác 0; ssh báo lỗi xác thực như thường lệ.
- **Prompt không phải prompt mật khẩu** (xác nhận host key, mã một lần): được chuyển tiếp tới
  terminal của người dùng; nếu không có terminal (port forward chạy nền) helper thoát mã khác 0 và
  ssh thất bại như hiện nay khi không có terminal.
- **OpenSSH cũ hơn 8.4** (không có `SSH_ASKPASS_REQUIRE`): ssh bỏ qua helper và hỏi mật khẩu trên
  terminal như trước; không có gì hỏng.
- **Có `master.key` nhưng metadata không có bí mật**: không thay đổi gì; key không bị xoay vòng.
- **Thiếu `master.key` và chưa có giá trị mã hoá nào**: được tạo ở lần lưu đầu tiên cần đến nó;
  không bao giờ được tạo bởi một thao tác chỉ đọc.
- **Pull khi thiếu `master.key`**: nếu dữ liệu pull về có bí mật thì tạo key mới, vì metadata pull về
  thay thế mọi giá trị mã hoá cũ.
- **`metadata.json` không giải mã được** (FR-008): chỉ server có giá trị lỗi bị khoá. Nó vẫn hiện
  trong danh sách, details pane ghi `(locked: <lỗi nêu tên file>)`, kết nối bằng mật khẩu bị huỷ
  với lỗi đó; server khác không bị ảnh hưởng. Pin/sửa server bị khoá mà không nhập bí mật mới thì
  giá trị mã hoá và file key đã tạo được giữ nguyên; nhập bí mật mới sẽ thay thế. Export/Push thất
  bại khi còn server bị khoá. Khôi phục bằng cách trả lại `master.key` hoặc Pull từ sync.
- **Đổi tên alias server**: mật khẩu đi theo server như metadata khác.
- **Copy SSH command** (`c`): không đổi; không bao giờ chứa mật khẩu.
- **Host `ProxyJump` cũng hỏi mật khẩu**: prompt của nó nêu một `user@host` khác, nên helper chuyển
  tiếp nó tới terminal thay vì trả lời bằng mật khẩu của server đích.
- **Sync bundle chứa giá trị `enc:v1:`** (bị sửa tay, hoặc do bản build lỗi tạo ra): Import thất
  bại trước khi ghi bất cứ gì với `sync bundle contains encrypted values from another machine`, vì
  `master.key` cục bộ không mở được chúng.

## Yêu cầu (Requirements) *(bắt buộc)*

### Yêu cầu chức năng (Functional Requirements)

#### Phương thức xác thực (form)

- **FR-001**: Tab Authentication PHẢI bắt đầu bằng dropdown `Auth method:` với các lựa chọn
  `identity file`, `embedded key`, `password`. Nó là công cụ hiển thị, không được lưu: khi mở, giá
  trị là `password` nếu server có mật khẩu, nếu không thì `embedded key` nếu có embedded key, nếu
  không thì `identity file`. Chọn một mục PHẢI chỉ bật các trường của phương thức đó và vô hiệu hoá
  các trường còn lại (`PrivateKey:` và `Import key from file:` cho `embedded key`; `Password:` cho
  `password`; `identity file` không bật trường nào, help text trỏ tới `Keys:` ở tab Basic).
- **FR-002**: `Password:` PHẢI là input field có che ký tự, hiện mật khẩu đã lưu ở dạng che khi
  form mở.
- **FR-003**: Khi lưu, bí mật của các phương thức không được chọn PHẢI bị xoá: phương thức
  `password` xoá embedded key (spec 001 sau đó xoá file key và mục `IdentityFile` của nó);
  `embedded key` và `identity file` xoá mật khẩu; `identity file` cũng xoá embedded key. Dropdown
  không bao giờ thay đổi `Keys:`.
- **FR-004**: Lưu phương thức `password` với mật khẩu rỗng PHẢI bị từ chối với
  `password is required`; mật khẩu chứa `\n` hoặc `\r` PHẢI bị từ chối với
  `password must be a single line`. Mật khẩu được lưu đúng như đã gõ (không trim).

#### Lưu trữ

- **FR-005**: Mật khẩu của server PHẢI được lưu trong `~/.lazyssh/metadata.json` dưới mục của
  server với khoá `password` (bỏ qua khi rỗng), cạnh `private_key` hiện có. Đây là dữ liệu riêng của
  lazyssh và KHÔNG ĐƯỢC ghi vào `~/.ssh/config`.
- **FR-006**: Cả `private_key` và `password` PHẢI được ghi vào `metadata.json` dạng
  `enc:v1:<base64(nonce || ciphertext AES-256-GCM)>` với nonce ngẫu nhiên mới cho mỗi giá trị, dùng
  key 32 byte trong `~/.lazyssh/master.key`. Không trường metadata nào khác được mã hoá.
- **FR-007**: `~/.lazyssh/master.key` PHẢI chứa 32 byte ngẫu nhiên, mode 0600 (thư mục 0700), chỉ
  được tạo khi cần mã hoá một giá trị và file chưa tồn tại, và không bao giờ bị ghi đè. File sai
  kích thước PHẢI được coi là hỏng.
- **FR-008**: Giá trị lưu không có tiền tố `enc:v1:` PHẢI được đọc như plaintext cũ và được ghi ở
  dạng mã hoá trong lần ghi `metadata.json` kế tiếp. Giá trị `enc:` có phiên bản lạ, base64 sai, xác
  thực thất bại, hoặc `master.key` thiếu/hỏng PHẢI khoá riêng server đó (`SecretError` nêu tên file
  có vấn đề: `~/.lazyssh/master.key` hoặc `~/.lazyssh/metadata.json` kèm alias), KHÔNG ĐƯỢC đưa
  ciphertext ra ngoài adapter, và mọi lần ghi sau đó PHẢI giữ nguyên giá trị mã hoá cũng như KHÔNG
  tạo `master.key` mới trừ khi có bí mật plaintext cần mã hoá.
- **FR-009**: Lỗi ở FR-008 PHẢI hiển thị cho người dùng: trong details pane của server bị khoá và
  trên status bar khi kết nối bằng mật khẩu thất bại (không chỉ ghi log).
- **FR-010**: Embedded key VẪN PHẢI được ghi ra `~/.lazyssh/keys/<alias>` (0600) dạng plaintext như
  spec 001, vì OpenSSH đọc file đó.

#### Kết nối bằng mật khẩu

- **FR-011**: Khi lazyssh khởi chạy `ssh` hệ thống cho server có mật khẩu (kết nối tương tác, kết
  nối với tham số forwarding, port forward chạy nền, phiên SFTP `ssh -s`), nó PHẢI đặt trong môi
  trường của tiến trình con: `SSH_ASKPASS=<đường dẫn tuyệt đối của file thực thi lazyssh đang
  chạy>`, `SSH_ASKPASS_REQUIRE=force`, `LAZYSSH_ASKPASS_ALIAS=<alias>`, và truyền các tuỳ chọn
  `-o PreferredAuthentications=password,keyboard-interactive -o NumberOfPasswordPrompts=1`. Bản
  thân mật khẩu KHÔNG ĐƯỢC xuất hiện trong tham số hay môi trường của bất kỳ tiến trình nào.
- **FR-012**: Server không có mật khẩu PHẢI được khởi chạy y hệt hiện nay (cùng tham số, không thêm
  biến môi trường).
- **FR-013**: Khi lazyssh khởi động với `LAZYSSH_ASKPASS_ALIAS` được đặt, nó PHẢI chạy như helper
  askpass thay vì TUI: nạp server theo alias đó, và nếu prompt (tham số đầu tiên) kết thúc bằng
  `password:` (không phân biệt hoa thường, bỏ qua khoảng trắng cuối) **và** chứa `<user>@<host>` của
  alias đó theo `ssh -G <alias>` (`user`, và `hostkeyalias` nếu có, nếu không thì `hostname`), in
  mật khẩu đã lưu kèm một ký tự xuống dòng ra stdout và thoát với mã 0. Prompt mật khẩu cho bất kỳ
  `user@host` nào khác (ví dụ host `ProxyJump`) PHẢI được xử lý như "prompt khác" (FR-014) để mật
  khẩu không bao giờ bị gửi tới host khác. Helper PHẢI không in gì ra stdout và thoát mã khác 0 khi
  alias không tồn tại, không có mật khẩu, hoặc metadata không đọc/giải mã được.
- **FR-014**: Với mọi prompt khác, helper PHẢI chuyển tiếp prompt tới terminal điều khiển
  (`/dev/tty`), đọc một dòng, in nó ra stdout và thoát mã 0; nếu không mở được terminal thì PHẢI
  thoát mã khác 0.
- **FR-015**: Helper KHÔNG ĐƯỢC khởi động TUI, KHÔNG ĐƯỢC ghi `metadata.json`, `master.key` hay
  file key, và KHÔNG ĐƯỢC ghi log mật khẩu hay câu trả lời prompt.

#### Sync

- **FR-016**: Định dạng sync bundle (spec 002, version 1) KHÔNG ĐƯỢC thay đổi. Export PHẢI đưa
  `private_key` và `password` vào `metadata` của bundle dạng plaintext (bản thân bundle đã được mã
  hoá bằng sync password); Import PHẢI mã hoá chúng bằng `master.key` cục bộ (tạo nếu thiếu) trước
  khi ghi `metadata.json`. `master.key` KHÔNG ĐƯỢC là một phần của bundle.
- **FR-017**: Export PHẢI thất bại (và push KHÔNG ĐƯỢC upload gì) nếu một giá trị không giải mã
  được (FR-008).

#### Vệ sinh bí mật

- **FR-018**: Mật khẩu và nội dung key KHÔNG ĐƯỢC xuất hiện trong log, status bar, details pane,
  thông báo lỗi, bộ lọc tìm kiếm, lệnh SSH được copy, hay test fixture trong repository; test PHẢI
  sinh mật khẩu và key lúc chạy trong `t.TempDir()`. Details pane PHẢI hiện ` (password)` sau danh
  sách key cho server có mật khẩu, giống ` (embedded)` cho key.

### Thực thể chính (Key Entities)

- **Server (mở rộng)**: thêm `Password` (plaintext trong bộ nhớ, rỗng khi không có). Phương thức
  xác thực được suy ra, không lưu: có password → password; nếu không, có embedded key → embedded
  key; nếu không → identity file.
- **Mục metadata của server**: thêm `password`; `private_key` và `password` chứa giá trị `enc:v1:`
  trên đĩa.
- **Master key**: `~/.lazyssh/master.key`, 32 byte ngẫu nhiên, riêng từng máy, không bao giờ sync.
- **Yêu cầu askpass**: alias (từ `LAZYSSH_ASKPASS_ALIAS`) và nội dung prompt (tham số 1) mà OpenSSH
  đưa cho helper.

## Tiêu chí thành công (Success Criteria) *(bắt buộc)*

### Kết quả đo được (Measurable Outcomes)

- **SC-001**: Người dùng có thể thêm một server chỉ dùng mật khẩu và kết nối với không lần hỏi mật
  khẩu nào, trên cả đường SSH lẫn SFTP.
- **SC-002**: Sau bất kỳ lần ghi nào, tìm theo byte trong `metadata.json` một mật khẩu đã lưu hoặc
  bất kỳ dòng nào của key đã lưu cho ra 0 kết quả.
- **SC-003**: Một `metadata.json` từ spec 001/002 nạp được với 0 bước thủ công và không còn bí mật
  plaintext nào sau lần ghi kế tiếp.
- **SC-004**: Push từ một máy và pull trên máy khác có `master.key` khác cho ra mật khẩu và key
  dùng được trên máy thứ hai ở 100% lần chạy test.
- **SC-005**: Quét trong bộ test các log, thông báo trạng thái, nội dung details pane, danh sách
  tham số ssh và môi trường tiến trình con tìm thấy mật khẩu đã sinh 0 lần.
- **SC-006**: Server không có mật khẩu khởi chạy với tham số và môi trường ssh giống hệt từng byte
  so với trước tính năng này.

## Giả định (Assumptions)

- Cần OpenSSH 8.4 trở lên để có hành vi không hỏi mật khẩu (`SSH_ASKPASS_REQUIRE`); client cũ hơn
  quay về hỏi trên terminal.
- `master.key` bảo vệ bí mật khi chỉ riêng `metadata.json` bị đọc (bản sao lưu, chia sẻ nhầm, nguồn
  sync bị lộ); nó không bảo vệ trước kẻ tấn công đọc được toàn bộ thư mục `~/.lazyssh` dưới quyền
  người dùng. Ai chạy được chương trình dưới quyền người dùng thì vốn đã đọc được cả hai file, nên
  helper askpass không tạo thêm rủi ro mới.
- Không hỗ trợ hạ cấp về bản lazyssh không có tính năng này sau khi bí mật đã được mã hoá.
- Ngoài phạm vi: OS keyring, master password hỏi lúc khởi động, mật khẩu cho ssh-agent hoặc
  passphrase của key, `sshpass`, SSH client nội bộ, xoay vòng `master.key`, hành vi askpass riêng
  cho Windows ngoài những gì OpenSSH cho Windows cung cấp.
