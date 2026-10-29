# Đặc tả tính năng: Sắp xếp file trong trình quản lý file SFTP

**Feature Branch**: `006-sftp-sort`

**Ngày tạo**: 2026-10-29

**Trạng thái**: Draft

**Input**: Mô tả của người dùng: "Trong file manager SFTP (màn hình `m`): (1) Thêm phím tắt sắp xếp
cho pane đang active: `s` đổi trường sắp xếp (Name ↔ Modified), `S` đảo chiều (↑/↓), giống màn hình
chính. Thư mục luôn đứng trước file bất kể chế độ. Chế độ hiện tại hiển thị trong title của pane (ví
dụ ` Remote: /srv (alias) [Modified ↓] `) và dòng status báo `Sort: <mode>` khi đổi. (2) Cột
Modified hiển thị đầy đủ theo định dạng `YYYY-MM-DD HH:MM:SS` thay cho `01-02` hiện tại."

**Xây dựng trên**: `specs/003-sftp-file-manager` (hai pane Local/Remote, dòng hints, phím tắt) và
`specs/005-sftp-filter` (filter `/` của từng pane, title có ` [/<chuỗi>]`).

## Clarifications

### Session 2026-10-29

Trả lời không tương tác (answered non-interactively) dựa trên các quyết định đi kèm yêu cầu tính năng (thời gian tạo, phạm vi pane, độ bền của chế độ, tie-break, múi giờ).

- Q: Có sắp xếp theo thời gian tạo (creation time) không? → A: Không. SFTP v3 và thông tin file của Go chỉ cung cấp thời gian sửa (mtime), không có thời gian tạo; chỉ sắp theo Name và Modified (Giả định).
- Q: Chế độ sắp xếp dùng chung hai pane hay riêng từng pane? → A: Riêng từng pane; `s`/`S` tác động lên pane đang active; mặc định `Name ↑` như hiện nay (FR-001, FR-002, FR-003).
- Q: Chế độ có giữ khi đổi thư mục, refresh, filter không? → A: Có, bền trong suốt phiên file manager; mọi lần nạp lại và lọc đều áp dụng chế độ hiện tại (FR-007).
- Q: Hai entry bằng nhau theo trường đang sắp thì xếp thế nào? → A: Cùng thời gian sửa thì theo tên tăng dần không phân biệt hoa thường; thư mục luôn trước file ở mọi chế độ, trong nhóm thư mục cũng sắp theo cùng trường/chiều (FR-004).
- Q: Cột Modified hiển thị theo múi giờ nào? → A: Giờ local của máy chạy lazyssh (FR-008).

## Kịch bản người dùng & Kiểm thử (User Scenarios & Testing) *(bắt buộc)*

### User Story 1 - Xem file mới sửa gần nhất (Priority: P1)

Người dùng mở file manager tới thư mục log trên server, bấm `s` rồi `S` ở pane Remote. Pane Remote
sắp theo thời gian sửa, mới nhất lên đầu (sau các thư mục); title thành
` Remote: /var/log (alias) [Modified ↓] ` và dòng status báo `Sort: Modified ↓`. Pane Local không đổi.

**Vì sao ưu tiên này**: Tìm file vừa sinh ra (log, bản build, backup) là lý do chính cần sắp theo
thời gian; thiếu nó phải đọc ngày từng dòng.

**Kiểm thử độc lập**: Nạp một pane với danh sách giả có thời gian sửa khác nhau, đổi chế độ, xác
nhận thứ tự hàng, title và thư mục vẫn đứng trước.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** pane đang ở `Name ↑`, **When** người dùng bấm `s`, **Then** pane chuyển sang
   `Modified ↑` (cũ nhất trước), thư mục vẫn đứng trước file, title kết thúc bằng ` [Modified ↑] `,
   dòng status báo `Sort: Modified ↑`.
2. **Given** pane đang ở `Modified ↑`, **When** người dùng bấm `S`, **Then** pane chuyển sang
   `Modified ↓` (mới nhất trước), thư mục vẫn đứng trước file.
3. **Given** pane đang ở `Modified ↓`, **When** người dùng bấm `s`, **Then** pane chuyển sang
   `Name ↓` (giữ chiều, đổi trường).
4. **Given** pane Remote đang active, **When** người dùng bấm `s`/`S`, **Then** pane Local giữ
   nguyên thứ tự và title.
5. **Given** hai entry cùng thời gian sửa, **When** sắp theo Modified ở bất kỳ chiều nào, **Then**
   hai entry xếp theo tên tăng dần không phân biệt hoa thường.

---

### User Story 2 - Xem thời gian sửa đầy đủ (Priority: P1)

Cột Modified hiện `2026-10-29 14:05:09` thay cho `10-29`, để người dùng phân biệt file cùng ngày
hoặc khác năm.

**Vì sao ưu tiên này**: Sắp theo thời gian mà chỉ thấy tháng-ngày thì không kiểm được thứ tự; định
dạng cũ cũng không cho biết năm.

**Kiểm thử độc lập**: Nạp pane với một entry có thời gian biết trước, đọc ô cột Modified của hàng
đó.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** một file có thời gian sửa 2026-10-29 14:05:09 (giờ local), **When** pane hiển thị file
   đó, **Then** ô Modified hiện `2026-10-29 14:05:09`.

---

### User Story 3 - Chế độ sắp xếp bền trong phiên (Priority: P2)

Sau khi chọn `Modified ↓`, người dùng vào thư mục con, refresh, lọc bằng `/`; pane vẫn sắp theo
`Modified ↓`.

**Vì sao ưu tiên này**: Phải bấm lại `s`/`S` sau mỗi lần đổi thư mục là phiền, nhưng Story 1 vẫn
dùng được khi thiếu.

**Kiểm thử độc lập**: Đặt chế độ, nạp thư mục khác và đặt filter, xác nhận thứ tự và title vẫn theo
chế độ đã chọn.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** pane ở `Modified ↓`, **When** người dùng vào thư mục khác, refresh (`R`) hoặc thao tác
   file làm pane nạp lại, **Then** danh sách mới vẫn sắp theo `Modified ↓`.
2. **Given** pane ở `Modified ↓` và filter `log`, **When** danh sách được lọc, **Then** các entry
   khớp giữ thứ tự `Modified ↓` và title có cả ` [Modified ↓]` lẫn ` [/log]`.

---

### Trường hợp biên (Edge Cases)

- Đổi chế độ khi đang có filter: filter giữ nguyên, chỉ thứ tự thay đổi.
- Đổi chế độ: con trỏ giữ nguyên số hàng (kẹp trong bảng), không nhảy về `../`.
- Hàng `../` luôn ở hàng đầu, không tham gia sắp xếp.
- Ô filter đang mở: gõ `s`/`S` là text của filter, không đổi chế độ.
- Mở lại file manager: cả hai pane trở về `Name ↑`.

## Yêu cầu (Requirements) *(bắt buộc)*

### Yêu cầu chức năng (Functional Requirements)

- **FR-001**: Mỗi pane PHẢI có chế độ sắp xếp riêng gồm trường (Name hoặc Modified) và chiều (↑ tăng
  dần, ↓ giảm dần); mặc định `Name ↑` khi mở file manager.
- **FR-002**: Bấm `s` khi focus ở bảng PHẢI đổi trường của pane đang active (Name ↔ Modified), giữ
  chiều.
- **FR-003**: Bấm `S` khi focus ở bảng PHẢI đảo chiều của pane đang active, giữ trường.
- **FR-004**: Ở mọi chế độ, thư mục PHẢI đứng trước file; trong mỗi nhóm, entry sắp theo trường và
  chiều hiện tại. Name so sánh không phân biệt hoa thường. Modified ↑ là cũ nhất trước, ↓ là mới
  nhất trước; hai entry cùng thời gian sửa xếp theo tên tăng dần không phân biệt hoa thường.
- **FR-005**: Title của pane PHẢI luôn có ` [<chế độ>]` (`Name ↑`, `Name ↓`, `Modified ↑`,
  `Modified ↓`) sau đường dẫn/alias và trước phần ` [/<filter>]` nếu có.
- **FR-006**: Khi đổi chế độ, dòng status PHẢI hiện `Sort: <chế độ>`; con trỏ giữ số hàng hiện tại
  (kẹp trong bảng); filter giữ nguyên.
- **FR-007**: Chế độ sắp xếp của pane PHẢI được áp dụng lại mỗi khi pane nạp danh sách (đổi thư mục,
  refresh, sau thao tác file) và khi lọc.
- **FR-008**: Cột Modified PHẢI hiển thị thời gian sửa theo định dạng `YYYY-MM-DD HH:MM:SS`, giờ
  local của máy chạy lazyssh.
- **FR-009**: Dòng hints PHẢI có thêm `s Sort` và `S Reverse`.

### Thực thể chính (Key Entities)

- **Chế độ sắp xếp của pane**: cặp (trường, chiều) gắn với một pane, tồn tại tới khi đóng file
  manager. Không lưu xuống đĩa.

## Tiêu chí thành công (Success Criteria) *(bắt buộc)*

### Kết quả đo được (Measurable Outcomes)

- **SC-001**: Từ một pane ở chế độ mặc định, người dùng đưa file mới sửa nhất lên đầu nhóm file với
  đúng 2 phím (`s`, `S`).
- **SC-002**: Thời gian sửa hiển thị chính xác tới giây cho 100% entry.
- **SC-003**: Danh sách sắp lại ngay sau khi bấm phím, không có độ trễ nhận thấy được với thư mục
  1.000 entry.
- **SC-004**: Không phím tắt hiện có nào của file manager đổi hành vi.

## Giả định (Assumptions)

- Ngoài phạm vi: sắp theo thời gian tạo (creation/birth time). Giao thức SFTP v3 mà server OpenSSH
  dùng chỉ trả về thời gian sửa (mtime) và thời gian truy cập, không có thời gian tạo; thông tin file
  của Go cũng chỉ có thời gian sửa. Hai pane phải hành xử giống nhau nên cũng không sắp theo thời
  gian tạo ở pane Local.
- Ngoài phạm vi: sắp theo kích thước, lưu chế độ sắp xếp giữa các phiên.
- Sắp xếp chỉ dựa trên danh sách đã nạp; không truy vấn lại server.
- Phần nối phím tcell được kiểm thủ công theo quickstart; logic sắp xếp, title, định dạng thời gian
  và giữ con trỏ được kiểm bằng unit test.
