# Đặc tả tính năng: Báo lỗi kết nối SSH/SFTP và xử lý host key đã đổi

**Feature Branch**: `009-ssh-connect-errors`

**Ngày tạo**: 2026-11-10

**Trạng thái**: Draft

**Input**: Mô tả của người dùng: "Khi kết nối vào một server mà ssh lỗi, ứng dụng không báo gì mà
quay thẳng về màn hình chính (ví dụ `WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!` ...
`Host key verification failed.`). Với trường hợp host key đổi: ứng dụng hỏi người dùng, cho phép
xác minh lại và kết nối lại. Với các lỗi kết nối khác: phải thông báo, hoặc đưa ra phương án xử
lý."

**Xây dựng trên**: kết nối SSH bằng phím Enter và chế độ `Forward + SSH` của port forwarding (gọi
`ssh` của hệ thống); `specs/003-sftp-file-manager` (file manager mở phiên SFTP qua `ssh`);
`specs/004-password-auth-encrypted-secrets` (mật khẩu đã lưu được đưa cho `ssh` qua askpass);
`specs/008-profile-home-dir`.

## Clarifications

### Session 2026-11-10

Trả lời không tương tác: mọi câu dưới đây **tự chốt theo phương án khuyến nghị, chưa được người
dùng xác nhận**; đổi bằng `/speckit-clarify`.

- Q: Làm sao phân biệt "kết nối lỗi" với "phiên kết thúc bằng mã khác 0"? → A: Chỉ khi ssh thoát
  với mã dành cho lỗi của chính nó (255) mới coi là lỗi kết nối; mọi mã khác (mã của shell/lệnh
  remote) và việc bị ngắt bằng tín hiệu (Ctrl-C) đều là thoát bình thường, không hộp thoại (FR-002).
  *(tự chốt, chưa được người dùng xác nhận)*
- Q: File manager SFTP có nằm trong phạm vi không? → A: Có. SFTP đã giữ sẵn phần cuối thông báo
  của ssh, nên dùng chung cách phân loại và hộp thoại (kể cả xoá key cũ rồi kết nối lại); chi phí
  nhỏ (FR-001). *(tự chốt, chưa được người dùng xác nhận)*
- Q: Hộp thoại host key đổi có những lựa chọn nào? → A: Đúng hai: `Cancel` (mặc định, Esc cũng là
  huỷ) và `Remove old key & reconnect`. Không có lựa chọn "chấp nhận key mới": việc chấp nhận luôn
  do ssh hỏi người dùng ở lần kết nối lại (FR-006, FR-008). *(tự chốt, chưa được người dùng xác
  nhận)*
- Q: Các lỗi thường có bộ nút khác nhau theo loại không? → A: Không, mọi lỗi thường dùng cùng bộ
  `Retry` / `Edit server` / `Close` (mặc định Close); gợi ý xử lý trong nội dung khác nhau theo loại
  (FR-005). *(tự chốt, chưa được người dùng xác nhận)*
- Q: Lỗi không do ssh báo (ví dụ không tìm thấy chương trình `ssh`, không đọc được mật khẩu đã
  lưu) xử lý thế nào? → A: Cũng hiện hộp thoại lỗi thường với nội dung lỗi đó làm nguyên nhân
  (FR-010). *(tự chốt, chưa được người dùng xác nhận)*

## Kịch bản người dùng & Kiểm thử (User Scenarios & Testing) *(bắt buộc)*

### User Story 1 - Host key của server đã đổi: cảnh báo, xác minh lại, kết nối lại (Priority: P1)

Server `web-prod` được cài lại nên có host key mới. Người dùng bấm Enter; ssh từ chối kết nối vì
key không khớp với `known_hosts`. Thay vì quay về danh sách trong im lặng, lazyssh hiện hộp thoại:
cảnh báo rằng đây có thể là tấn công xen giữa (man-in-the-middle), hiển thị fingerprint của key
mới mà server gửi, file và số dòng chứa key cũ, và hỏi có xoá key cũ rồi kết nối lại không. Nút
được chọn sẵn là huỷ. Người dùng đã xác nhận với quản trị viên rằng server vừa được cài lại, chọn
xoá và kết nối lại; ssh hỏi "Are you sure you want to continue connecting?" kèm fingerprint mới,
người dùng đối chiếu rồi gõ `yes` và vào được shell.

**Vì sao ưu tiên này**: Đây là tình huống người dùng gặp thật; hiện tại họ không biết vì sao không
vào được và phải tự tìm lệnh sửa `known_hosts`. Đồng thời là tình huống nhạy cảm bảo mật nhất.

**Kiểm thử độc lập**: Dùng một server thử, ghi host key cũ vào `known_hosts`, đổi host key của
server, kết nối từ lazyssh; kiểm tra nội dung hộp thoại, chọn Cancel thì `known_hosts` không đổi,
chọn xoá thì key cũ bị xoá và ssh hỏi xác nhận key mới.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** host key của server khác với key trong `known_hosts`, **When** người dùng SSH (Enter
   hoặc Forward + SSH) hoặc mở file manager, **Then** hiện hộp thoại có: cảnh báo có thể bị tấn
   công xen giữa, tên host như trong `known_hosts`, loại key và fingerprint mới, file:dòng của key
   cũ; nút được chọn sẵn là huỷ.
2. **Given** hộp thoại host key đổi đang mở, **When** người dùng chọn Cancel hoặc bấm Esc,
   **Then** quay về danh sách server, `known_hosts` không thay đổi.
3. **Given** hộp thoại host key đổi đang mở, **When** người dùng chọn xoá key cũ và kết nối lại,
   **Then** mọi key của host đó trong đúng file `known_hosts` được nêu bị xoá (bản cũ được giữ lại
   dạng `<file>.old`), rồi lazyssh kết nối lại theo đúng cách ban đầu (SSH hoặc SFTP) và ssh hỏi
   người dùng xác nhận fingerprint mới trên terminal.
4. **Given** ở bước 3 người dùng trả lời `no` cho câu hỏi của ssh, **When** ssh dừng, **Then**
   lazyssh hiện hộp thoại lỗi "host key chưa được xác minh" (US2), không tự chấp nhận key.
5. **Given** việc xoá key cũ thất bại (ví dụ file chỉ đọc hoặc là file hệ thống), **When** người
   dùng đã chọn xoá, **Then** hiện thông báo lỗi kèm lý do và KHÔNG kết nối lại.
6. **Given** server dùng cổng khác 22, **When** xoá key cũ, **Then** tên host được dùng đúng dạng
   `[host]:port` như ssh đã báo, không đoán từ alias.

---

### User Story 2 - Lỗi kết nối khác: nói rõ nguyên nhân và cách xử lý (Priority: P1)

Người dùng bấm Enter vào một server đang tắt. Thay vì màn hình nháy rồi quay lại danh sách,
lazyssh hiện hộp thoại "Connection refused" với gợi ý kiểm tra dịch vụ SSH/cổng, vài dòng cuối
mà ssh in ra, và các lựa chọn Retry / Edit server / Close.

**Vì sao ưu tiên này**: Mọi lỗi kết nối hiện đều bị nuốt; người dùng không thể tự xử lý điều họ
không nhìn thấy.

**Kiểm thử độc lập**: Kết nối tới cổng không có dịch vụ, tới hostname không tồn tại, bằng key sai,
v.v.; mỗi lần đều thấy hộp thoại với nguyên nhân đúng và nút hoạt động.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** ssh không kết nối được, **When** ssh dừng, **Then** hiện hộp thoại gồm: nguyên nhân
   ngắn gọn, gợi ý xử lý, các dòng cuối ssh đã in ra (nguyên văn), và các nút Retry / Edit server /
   Close; nút được chọn sẵn là Close.
2. **Given** lỗi thuộc các loại sau, **Then** nguyên nhân hiển thị tương ứng:
   - Từ chối xác thực / quá số lần xác thực → "Authentication failed".
   - Cổng không có dịch vụ → "Connection refused".
   - Hết thời gian chờ → "Connection timed out".
   - Không phân giải được hostname → "Host name not found".
   - Không có đường tới host / mạng không tới được → "Host unreachable".
   - File private key có quyền quá rộng → "Private key permissions too open".
   - Host key chưa được xác minh (người dùng trả lời `no`, hoặc chưa biết host trong chế độ kiểm
     tra nghiêm ngặt) → "Host key not verified".
   - Không đọc được mật khẩu đã lưu → "Stored password unavailable" (lỗi xảy ra trước khi chạy ssh).
3. **Given** lỗi không thuộc loại nào đã biết, **When** ssh dừng vì lỗi, **Then** vẫn hiện hộp
   thoại với nguyên nhân chung và nguyên văn các dòng cuối ssh đã in; nếu ssh không in gì thì nêu
   rõ mã thoát.
4. **Given** hộp thoại lỗi đang mở, **When** chọn Retry, **Then** kết nối lại cùng server theo cùng
   cách (SSH, Forward + SSH với cùng tham số, hoặc SFTP); **When** chọn Edit server, **Then** mở
   form sửa server đó; **When** chọn Close hoặc Esc, **Then** về danh sách server.

---

### User Story 3 - Thoát phiên bình thường không bị làm phiền (Priority: P1)

Người dùng SSH vào server, chạy vài lệnh, lệnh cuối lỗi rồi gõ `exit` (hoặc bấm Ctrl-C ở dấu nhắc
mật khẩu để huỷ). Lazyssh quay về danh sách như hiện tại, không hiện hộp thoại nào.

**Vì sao ưu tiên này**: Mã thoát khác 0 sau một phiên thành công là chuyện thường ngày; hộp thoại
lỗi ở đây sẽ là báo động giả.

**Kiểm thử độc lập**: Kết nối thành công, chạy `exit 3`; kết nối rồi huỷ ở dấu nhắc mật khẩu; cả
hai đều quay về danh sách không có hộp thoại.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** phiên SSH đã mở, **When** shell/lệnh remote kết thúc với bất kỳ mã thoát nào mà chính
   ssh không báo lỗi, **Then** không hiện hộp thoại.
2. **Given** ssh đang hỏi mật khẩu/passphrase, **When** người dùng bấm Ctrl-C, **Then** không hiện
   hộp thoại.
3. **Given** kết nối thành công, **Then** mọi dấu nhắc tương tác của ssh (mật khẩu, passphrase,
   xác nhận host key) và thông báo của ssh vẫn hiện trên terminal như trước tính năng.

---

### Trường hợp biên (Edge Cases)

- Kết nối bị đứt giữa phiên (server khởi động lại, mạng rớt): ssh báo lỗi của chính nó nên hiện
  hộp thoại với các dòng cuối ssh in ra; người dùng có thể Retry.
- Lệnh remote tự thoát với đúng mã mà ssh dùng cho lỗi của nó: không phân biệt được, sẽ hiện hộp
  thoại "lỗi không nhận diện được" kèm nguyên văn đầu ra (chấp nhận).
- ssh in rất nhiều dòng (ví dụ chạy với `-v` qua `LogLevel` trong config): chỉ giữ và hiện phần
  cuối, không làm chậm hay tốn bộ nhớ không giới hạn.
- Thông báo của ssh chứa ký tự mà giao diện coi là định dạng (ví dụ `[127.0.0.1]:2222`): phải hiện
  nguyên văn.
- Host key đổi nhưng thông báo của ssh không cho biết tên host (ví dụ `LogLevel` quá thấp): hộp
  thoại vẫn cảnh báo nhưng không đề nghị xoá key, chỉ hướng dẫn tự sửa `known_hosts`.
- Key cũ nằm trong file `known_hosts` hệ thống không ghi được: việc xoá thất bại và được báo lỗi
  (US1-5).
- Server có `HostKeyAlias`: tên được xoá là tên ssh báo (alias khoá), không phải alias lazyssh.
- Một phiên `ssh` dùng `ControlPersist` để lại tiến trình master chạy nền: lazyssh vẫn quay về
  danh sách ngay khi phiên kết thúc.

## Yêu cầu (Requirements) *(bắt buộc)*

### Yêu cầu chức năng (Functional Requirements)

- **FR-001**: Khi kết nối SSH (Enter), Forward + SSH, hoặc mở file manager thất bại vì chính ssh
  báo lỗi, lazyssh PHẢI hiện hộp thoại thay vì quay về danh sách trong im lặng.
- **FR-002**: Phiên SSH kết thúc với mã thoát của shell/lệnh remote, hoặc bị người dùng ngắt bằng
  tín hiệu (Ctrl-C), KHÔNG được hiện hộp thoại.
- **FR-003**: Mọi thông báo và dấu nhắc của ssh PHẢI vẫn hiện trên terminal trong lúc kết nối SSH
  như trước; lazyssh chỉ giữ thêm một bản sao giới hạn (phần cuối) để phân loại lỗi.
- **FR-004**: Lazyssh PHẢI nhận diện ít nhất các loại lỗi ở US2-2 và host key đổi (US1) từ đúng
  thông báo của OpenSSH, và PHẢI có nguyên nhân chung cho lỗi không nhận diện được.
- **FR-005**: Hộp thoại lỗi thường PHẢI hiện nguyên nhân, gợi ý xử lý, tối đa vài dòng cuối của
  thông báo ssh (hiện nguyên văn, không bị diễn giải thành định dạng) và nút Retry / Edit server /
  Close với Close là mặc định.
- **FR-006**: Hộp thoại host key đổi PHẢI hiện cảnh báo tấn công xen giữa, tên host, loại key và
  fingerprint mới, file:dòng của key cũ; chỉ có hai nút `Cancel` (mặc định; Esc tương đương) và
  `Remove old key & reconnect`.
- **FR-007**: Chỉ khi người dùng chọn rõ ràng hành động xoá, lazyssh mới xoá các key của đúng tên
  host mà ssh báo, trong đúng file `known_hosts` mà ssh báo chứa key cũ, rồi kết nối lại theo cùng
  cách ban đầu.
- **FR-008**: Lazyssh KHÔNG BAO GIỜ tự chấp nhận key mới, không tắt hay nới việc kiểm tra host key
  của ssh; việc xác nhận key mới luôn do người dùng trả lời câu hỏi của ssh.
- **FR-009**: Nếu xoá key thất bại, PHẢI báo lỗi kèm lý do và không kết nối lại.
- **FR-010**: Lỗi xảy ra trước khi ssh chạy (không đọc được mật khẩu đã lưu, không tìm thấy
  chương trình `ssh`) PHẢI được báo bằng cùng hộp thoại lỗi thường; thông báo không được chứa mật
  khẩu hay khoá.
- **FR-011**: Các chuỗi giao diện bằng tiếng Anh như phần còn lại của ứng dụng.
- **FR-012**: Vì stderr của ssh có thể chứa văn bản do server quyết định (banner trước xác thực,
  output của phiên không có tty), lazyssh CHỈ đề nghị xoá key khi tên host trong cảnh báo trùng
  với tên mà ssh dùng trong `known_hosts` cho chính server đang kết nối (`HostKeyAlias`, hoặc
  `HostName` kèm `Port` theo `ssh -G`). Không trùng (kể cả host key của jump host đổi) thì chỉ
  cảnh báo, người dùng tự sửa `known_hosts`. *(bổ sung khi review cuối, 2026-11-10)*

### Thực thể chính (Key Entities)

- **Lỗi kết nối SSH**: nguyên nhân (một dòng), gợi ý xử lý, các dòng cuối ssh in ra; với host key
  đổi thì có thêm tên host trong `known_hosts`, file và dòng của key cũ, loại key và fingerprint
  mới. Không lưu bền; chỉ tồn tại cho đến khi hộp thoại đóng.

## Tiêu chí thành công (Success Criteria) *(bắt buộc)*

### Kết quả đo được (Measurable Outcomes)

- **SC-001**: 100% lần kết nối thất bại vì lỗi của ssh (SSH, Forward + SSH, SFTP) đều hiện hộp
  thoại; 0 lần quay về danh sách trong im lặng.
- **SC-002**: 0 hộp thoại sau phiên SSH kết thúc bình thường (kể cả mã thoát remote khác 0) hoặc
  bị người dùng huỷ.
- **SC-003**: Từ lúc thấy cảnh báo host key đổi, người dùng kết nối lại được (sau khi xác minh
  fingerprint) chỉ với 1 lựa chọn trong hộp thoại và 1 câu trả lời cho ssh, không cần tự sửa file.
- **SC-004**: 0 trường hợp `known_hosts` bị thay đổi khi người dùng chưa chọn xoá; 0 trường hợp key
  mới được chấp nhận mà không qua câu hỏi của ssh.
- **SC-005**: Với mỗi loại lỗi ở US2-2, hộp thoại nêu đúng nguyên nhân (kiểm bằng mẫu thông báo
  thật của OpenSSH).

## Giả định (Assumptions)

- Client là OpenSSH; các thông báo được nhận diện là thông báo tiếng Anh của OpenSSH. Client khác
  hoặc thông báo khác rơi vào nhánh "lỗi không nhận diện được" (vẫn hiện nguyên văn).
- ssh dùng mã thoát riêng cho lỗi của chính nó, khác mã thoát của lệnh remote thông thường; đó là
  cách phân biệt "kết nối lỗi" với "phiên kết thúc".
- Việc xoá key cũ dùng công cụ quản lý `known_hosts` đi kèm OpenSSH, công cụ này tự giữ bản sao
  `<file>.old`.
- Port forwarding chạy nền (`Only forward`) chỉ báo trên status bar khi không khởi chạy được tiến
  trình; lỗi kết nối của tiến trình nền (chạy không có terminal) ngoài phạm vi tính năng này.
- Bấm Ctrl-C khi ssh đang hỏi (mật khẩu, host key) hiện làm cả lazyssh thoát vì lazyssh và ssh
  cùng nhận tín hiệu ngắt; hành vi này có từ trước tính năng (đã kiểm trên `main`) và ngoài phạm vi.
- Sau mỗi lần kết nối, danh sách server được nạp lại và con trỏ về dòng đầu (có từ trước); các nút
  Retry / Edit server / xoá key luôn dùng đúng server vừa kết nối, không phụ thuộc con trỏ.
- Ngoài phạm vi: tự sửa quyền file key, tự thêm key mới vào `known_hosts`, so sánh fingerprint với
  nguồn bên ngoài (DNS SSHFP...), dịch thông báo sang ngôn ngữ khác, tuỳ chọn cấu hình bật/tắt
  hộp thoại.
