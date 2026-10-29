# Đặc tả tính năng: Thư mục làm việc mặc định (Home Dir) cho mỗi profile SSH

**Feature Branch**: `008-profile-home-dir`

**Ngày tạo**: 2026-10-29

**Trạng thái**: Draft

**Input**: Mô tả của người dùng: "Mỗi profile SSH có thêm trường cấu hình "home dir" (thư mục làm
việc mặc định trên máy remote). Sau khi kết nối SSH hoặc SFTP thành công, tự động cd tới thư mục
này."

**Xây dựng trên**: form Add/Edit server và pane chi tiết hiện có; `specs/003-sftp-file-manager`
(pane Remote mở tại thư mục làm việc của phiên SFTP); `specs/002-encrypted-sync` (metadata lazyssh
được đồng bộ cùng SSH config).

## Clarifications

### Session 2026-10-29

Trả lời không tương tác (answered non-interactively) theo mặc định hợp lý; người dùng có thể đổi
bằng `/speckit-clarify`.

- Q: Profile đã có `RemoteCommand` thì Home Dir xử lý thế nào khi SSH? → A: Vẫn áp dụng: lệnh của
  người dùng được chạy từ Home Dir (chuyển thư mục trước rồi chạy `RemoteCommand`). Giá trị
  `RemoteCommand=none` được coi như không có lệnh (FR-004).
- Q: Thư mục không tồn tại hoặc không có quyền truy cập trên server? → A: SSH: vẫn mở shell tại
  thư mục mặc định của tài khoản remote, lỗi chuyển thư mục hiện ngay trên terminal trước dấu nhắc.
  SFTP: pane Remote mở tại thư mục làm việc mặc định của phiên và dòng status báo đỏ
  `Home dir not found: <dir>` (FR-005, FR-008).
- Q: Lưu ở đâu? → A: Home Dir không phải tuỳ chọn của OpenSSH nên lưu trong metadata riêng của
  lazyssh (`~/.lazyssh/metadata.json`), không ghi vào `~/.ssh/config`; nhờ đó được đồng bộ cùng
  metadata hiện có (FR-002, FR-003).
- Q: Lệnh copy bằng phím `c` có kèm bước chuyển thư mục không? → A: Không. Lệnh copy là lệnh
  `ssh` thuần để dùng ngoài lazyssh; ngoài phạm vi tính năng này (Giả định).

## Kịch bản người dùng & Kiểm thử (User Scenarios & Testing) *(bắt buộc)*

### User Story 1 - SSH vào thẳng thư mục dự án (Priority: P1)

Người dùng sửa profile `web-prod`, điền `Home Dir: /var/www/app`, lưu. Lần sau bấm Enter để SSH,
shell trên server mở sẵn tại `/var/www/app` thay vì thư mục home của tài khoản; người dùng gõ lệnh
ngay, không cần `cd`.

**Vì sao ưu tiên này**: Đây là mục đích chính của tính năng; hầu hết phiên SSH bắt đầu bằng một
lệnh `cd` lặp đi lặp lại.

**Kiểm thử độc lập**: Đặt Home Dir cho một server, kết nối SSH, chạy `pwd` thấy đúng thư mục; xoá
Home Dir, kết nối lại, `pwd` trả về thư mục mặc định.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** profile có Home Dir `/var/www/app` tồn tại trên server, **When** người dùng SSH (phím
   Enter, hoặc SSH kèm port forwarding), **Then** shell tương tác mở tại `/var/www/app`.
2. **Given** profile có Home Dir rỗng, **When** người dùng SSH, **Then** hành vi giống hiện tại
   (shell tại thư mục mặc định, không thêm tham số nào vào lệnh ssh).
3. **Given** Home Dir là `~/projects/app`, **When** SSH, **Then** shell mở tại
   `<home của user remote>/projects/app`.
4. **Given** Home Dir không tồn tại trên server, **When** SSH, **Then** shell vẫn mở tại thư mục
   mặc định và terminal hiện thông báo lỗi chuyển thư mục trước dấu nhắc.
5. **Given** profile có `RemoteCommand` (ví dụ `tmux attach || tmux new`) và Home Dir, **When**
   SSH, **Then** lệnh đó chạy với thư mục làm việc là Home Dir.

---

### User Story 2 - SFTP mở pane Remote tại Home Dir (Priority: P2)

Người dùng bấm `m` để mở file manager cho `web-prod`; pane Remote hiển thị ngay nội dung
`/var/www/app`, pane Local không đổi.

**Vì sao ưu tiên này**: Cùng một cấu hình phục vụ cả hai cách kết nối; tiết kiệm nhiều lần điều
hướng thư mục trong file manager.

**Kiểm thử độc lập**: Đặt Home Dir, mở file manager, nhìn tiêu đề/đường dẫn pane Remote.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** profile có Home Dir tồn tại trên server, **When** mở file manager, **Then** pane
   Remote nạp thư mục đó; pane Local vẫn ở home cục bộ.
2. **Given** Home Dir rỗng, **When** mở file manager, **Then** pane Remote nạp thư mục làm việc
   mặc định của phiên như hiện tại.
3. **Given** Home Dir không tồn tại hoặc không đọc được, **When** mở file manager, **Then** pane
   Remote nạp thư mục mặc định và dòng status báo đỏ `Home dir not found: <dir>`.

---

### User Story 3 - Cấu hình và xem Home Dir trong giao diện (Priority: P2)

Trong form Add/Edit server có ô `Home Dir:` với gợi ý ví dụ; pane chi tiết hiển thị Home Dir khi
có giá trị. Giá trị được giữ qua các lần sửa profile khác và qua đồng bộ (sync).

**Vì sao ưu tiên này**: Không có chỗ nhập thì hai story trên không dùng được; nhưng tách riêng để
kiểm thử lưu/đọc độc lập với việc kết nối.

**Kiểm thử độc lập**: Thêm server với Home Dir, thoát và mở lại lazyssh, mở form Edit thấy giá trị
còn nguyên; sửa trường khác (ví dụ Tags) không làm mất Home Dir.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** form Add server, **When** nhập Home Dir và lưu, **Then** mở lại form Edit thấy đúng
   giá trị, và `~/.ssh/config` không có dòng nào về Home Dir.
2. **Given** server có Home Dir, **When** sửa và lưu một trường khác, **Then** Home Dir giữ nguyên.
3. **Given** server có Home Dir, **When** xoá trắng ô Home Dir và lưu, **Then** các lần kết nối sau
   không chuyển thư mục nữa.
4. **Given** server có Home Dir, **When** xem pane chi tiết, **Then** thấy dòng `Home Dir` với giá
   trị; server không có Home Dir thì không hiện dòng này.
5. **Given** hai máy dùng sync, **When** máy A đặt Home Dir và push, máy B pull, **Then** máy B có
   cùng Home Dir cho server đó.

---

### Trường hợp biên (Edge Cases)

- Đường dẫn có khoảng trắng hoặc ký tự đặc biệt của shell (`'`, `$`, `;`): PHẢI được chuyển tới
  đúng thư mục có tên đó, không bị tách hay thực thi như lệnh.
- `~` hoặc `~/...`: hiểu là home của tài khoản remote (cả SSH lẫn SFTP). Đường dẫn tương đối không
  có `~` cũng tính từ home của tài khoản remote.
- Khoảng trắng đầu/cuối giá trị bị cắt bỏ; giá trị chỉ gồm khoảng trắng coi như rỗng.
- Giá trị chứa xuống dòng: bị từ chối khi lưu với thông báo lỗi.
- Profile cũ chưa có trường này trong metadata: đọc như rỗng, không lỗi.
- `RemoteCommand=none`: coi như không có lệnh; Home Dir vẫn áp dụng với shell tương tác.
- Server có `RequestTTY=no`: phiên SSH mở từ lazyssh với Home Dir vẫn là shell tương tác (yêu cầu
  terminal) như khi không có Home Dir.

## Yêu cầu (Requirements) *(bắt buộc)*

### Yêu cầu chức năng (Functional Requirements)

- **FR-001**: Form Add/Edit server PHẢI có ô nhập `Home Dir:` (một dòng, có placeholder ví dụ và
  mô tả trợ giúp như các trường khác); giá trị được cắt khoảng trắng hai đầu; giá trị chứa ký tự
  xuống dòng PHẢI bị từ chối khi lưu.
- **FR-002**: Home Dir PHẢI được lưu bền trong metadata riêng của lazyssh theo alias, KHÔNG ghi
  vào SSH config; giá trị rỗng không tạo khoá trong file metadata.
- **FR-003**: Home Dir PHẢI được giữ nguyên khi sửa các trường khác của profile, khi đổi alias (đi
  theo profile), và PHẢI nằm trong dữ liệu được sync push/pull.
- **FR-004**: Khi profile có Home Dir khác rỗng, mọi phiên SSH tương tác mở từ lazyssh (phím
  Enter, SSH kèm port forwarding) PHẢI bắt đầu với thư mục làm việc là Home Dir: nếu profile có
  `RemoteCommand` (khác rỗng và khác `none`) thì lệnh đó chạy từ Home Dir; nếu không thì mở login
  shell của tài khoản remote tại Home Dir.
- **FR-005**: Khi chuyển thư mục thất bại ở phiên SSH, phiên PHẢI vẫn tiếp tục tại thư mục mặc
  định của tài khoản remote và lỗi PHẢI hiện trên terminal.
- **FR-006**: Khi Home Dir rỗng, lệnh ssh được chạy PHẢI giống hệt hiện tại.
- **FR-007**: Khi mở file manager SFTP cho profile có Home Dir, pane Remote PHẢI nạp Home Dir
  (sau khi thay `~`/đường dẫn tương đối bằng home của tài khoản remote); pane Local không đổi.
- **FR-008**: Nếu Home Dir không nạp được ở SFTP, pane Remote PHẢI nạp thư mục mặc định của phiên
  và dòng status PHẢI báo đỏ `Home dir not found: <dir>`.
- **FR-009**: Pane chi tiết server PHẢI hiển thị dòng `Home Dir` khi giá trị khác rỗng.
- **FR-010**: Đường dẫn chứa khoảng trắng hoặc ký tự đặc biệt của shell PHẢI được xử lý như tên
  thư mục nguyên văn (không bị tách, không bị diễn giải thành lệnh), ngoại trừ `~` ở đầu được hiểu
  là home remote.
- **FR-011**: Giá trị Home Dir không phải bí mật và có thể xuất hiện trong log ở mức debug cùng
  alias; không được log kèm mật khẩu hay khoá.

### Thực thể chính (Key Entities)

- **Profile SSH (Server)**: thêm thuộc tính `Home Dir` (chuỗi, tuỳ chọn): đường dẫn thư mục trên
  máy remote dùng làm thư mục làm việc ban đầu cho SSH và SFTP. Lưu trong metadata lazyssh theo
  alias, cùng chỗ với Tags/PinnedAt.

## Tiêu chí thành công (Success Criteria) *(bắt buộc)*

### Kết quả đo được (Measurable Outcomes)

- **SC-001**: Với profile đã đặt Home Dir tồn tại trên server, 100% phiên SSH mở từ lazyssh có
  `pwd` ban đầu bằng Home Dir; người dùng không gõ thêm lệnh nào.
- **SC-002**: Với profile đã đặt Home Dir, pane Remote của file manager hiển thị Home Dir ngay khi
  mở, không cần điều hướng.
- **SC-003**: Với profile không đặt Home Dir, lệnh ssh được chạy và thư mục mở đầu của SFTP không
  thay đổi so với trước tính năng (0 khác biệt).
- **SC-004**: Home Dir sống sót qua: thoát/mở lại ứng dụng, sửa trường khác, đổi alias, sync
  push/pull (100% trường hợp trong kiểm thử).
- **SC-005**: `~/.ssh/config` không chứa thêm dòng nào do tính năng này tạo ra.

## Giả định (Assumptions)

- Tài khoản remote có shell đăng nhập dạng POSIX (`sh`, `bash`, `zsh`...) để lệnh chuyển thư mục
  rồi mở shell hoạt động; shell không chuẩn (Windows `cmd`) ngoài phạm vi.
- Phiên SSH mở từ lazyssh luôn là phiên tương tác có terminal; việc yêu cầu terminal khi có Home Dir
  không làm thay đổi trải nghiệm so với hiện tại.
- Lệnh copy bằng phím `c` giữ nguyên, không kèm bước chuyển thư mục.
- Lớp service chịu trách nhiệm tạo tham số ssh từ Home Dir và `RemoteCommand` (có unit test cho
  quoting, `~`, rỗng, kết hợp RemoteCommand); giao diện chỉ thêm ô nhập, hiển thị chi tiết và truyền
  Home Dir cho file manager. Phần kết nối thật được kiểm thủ công theo quickstart.
- Ngoài phạm vi: Home Dir cho pane Local, nhiều thư mục/bookmark cho một profile, tự ghi nhớ thư mục
  cuối cùng đã đứng, kiểm tra tồn tại thư mục lúc lưu form.
