# Đặc tả tính năng: Xem/sửa file bằng editor trong trình quản lý file SFTP

**Feature Branch**: `007-sftp-edit`

**Ngày tạo**: 2026-10-29

**Trạng thái**: Draft

**Input**: Mô tả của người dùng: "Trong file manager SFTP (màn hình `m`), thêm phím `e` để xem/sửa
file đang chọn ở pane active bằng editor dòng lệnh (`vi`): (1) Pane Local: tạm dừng TUI, chạy editor
với đường dẫn file, kế thừa terminal; khi editor thoát thì quay lại file manager và reload pane. (2)
Pane Remote: tải file xuống một file tạm cục bộ (thư mục tạm riêng, file mode 0600), chạy editor trên
file tạm; khi editor thoát, nếu nội dung thay đổi thì upload ghi đè file remote, báo `Saved <name>`
(xanh); nếu không đổi thì báo `No changes`; xoá thư mục tạm trong mọi trường hợp; reload pane remote.
(3) Chỉ áp dụng cho file thường (không phải thư mục, không phải hàng `../`); bấm `e` trên thư mục thì
không làm gì. (4) Lỗi (editor không tìm thấy, editor thoát mã khác 0, tải xuống/tải lên thất bại)
hiện ở dòng status màu đỏ: `Edit failed: <err>`; khi upload thất bại, file tạm được giữ lại và đường
dẫn của nó được đưa vào thông báo để người dùng không mất bản sửa."

**Xây dựng trên**: `specs/003-sftp-file-manager` (hai pane Local/Remote, dòng hints, phím tắt, cơ chế
một transfer tại một thời điểm), `specs/005-sftp-filter` và `specs/006-sftp-sort`.

## Clarifications

### Session 2026-10-29

Trả lời không tương tác (answered non-interactively) dựa trên các quyết định đi kèm yêu cầu tính năng (chọn editor, phát hiện thay đổi, cách ghi lại file remote, tương tác với transfer đang chạy, phân tách trách nhiệm).

- Q: Editor được chọn như thế nào? → A: Dùng biến môi trường `EDITOR` nếu khác rỗng, tách thành chương trình và tham số theo khoảng trắng (cho phép `code --wait`); ngược lại dùng `vi`. (FR-011).
- Q (bổ sung 2026-10-29): Người dùng muốn đổi `vi` thành `vim`/`nvim` mà không đụng biến môi trường? → A: Thêm trường `Editor:` vào form Sync & Settings, lưu vào `settings.json` khoá `editor`; giá trị này ưu tiên hơn `EDITOR` (FR-011).
- Q: Làm sao biết người dùng đã đổi nội dung file remote? → A: So sánh dấu vân tay SHA-256 của nội dung bản sao tạm trước và sau khi editor chạy; không dựa vào thời gian sửa vì editor có thể ghi lại cùng nội dung (FR-005, FR-006).
- Q: Ghi lại file remote thế nào, có giới hạn kích thước không? → A: Mở file remote ở chế độ ghi đè (cắt về 0 rồi ghi) và chép nội dung bản sao lên; không giới hạn kích thước, file lớn chỉ tốn thời gian tải; ghi đè tại chỗ nên quyền truy cập của file remote giữ nguyên (FR-005).
- Q: Bấm `e` khi đang có transfer chạy thì sao? → A: Pane Local được phép. Pane Remote dùng chung cơ chế "một transfer tại một thời điểm": nếu đang bận thì báo đỏ `A transfer is already running` và không mở editor; trong lúc sửa remote, trạng thái bận được bật để transfer khác không chạy song song trên cùng phiên (FR-012).
- Q: Logic nào thuộc lớp service, logic nào thuộc giao diện? → A: Lớp service có hàm kiểm thử được lo tải xuống bản sao tạm, gọi editor (được truyền vào), so sánh dấu vân tay, tải lên và dọn dẹp, cùng một hàm nhỏ chọn lệnh editor từ giá trị `EDITOR`. Giao diện chỉ nối phím `e`, tạm dừng TUI, chạy editor với terminal kế thừa và hiển thị kết quả (Giả định).

## Kịch bản người dùng & Kiểm thử (User Scenarios & Testing) *(bắt buộc)*

### User Story 1 - Sửa nhanh file cấu hình trên server (Priority: P1)

Người dùng mở file manager tới `/etc/nginx` trên server, chọn `nginx.conf` ở pane Remote và bấm `e`.
Màn hình TUI nhường chỗ cho editor đang mở bản sao của file. Người dùng sửa, lưu, thoát editor; file
manager quay lại, file trên server đã được ghi đè bằng nội dung mới và dòng status báo
`Saved nginx.conf` màu xanh.

**Vì sao ưu tiên này**: Sửa file trên server là việc thường gặp nhất sau khi duyệt file; hiện phải
tải xuống, mở editor ngoài, rồi tải lên lại bằng tay.

**Kiểm thử độc lập**: Với một file remote có nội dung biết trước, chạy luồng sửa với một "editor"
giả ghi thêm nội dung, xác nhận file remote đổi, thông báo đúng và không còn file tạm.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** pane Remote đang active và con trỏ ở một file thường, **When** người dùng bấm `e`, sửa
   và lưu trong editor rồi thoát, **Then** file remote mang nội dung mới, giữ nguyên quyền truy cập
   cũ, dòng status báo `Saved <name>` (xanh), pane Remote được nạp lại.
2. **Given** như trên, **When** người dùng thoát editor mà không đổi nội dung, **Then** file remote
   không bị ghi lại và dòng status báo `No changes`.
3. **Given** luồng sửa remote đã kết thúc (thành công, không đổi, hoặc lỗi trước bước tải lên),
   **When** kiểm tra máy cục bộ, **Then** không còn bản sao tạm nào của file.
4. **Given** tải lên thất bại sau khi người dùng đã sửa, **When** file manager quay lại, **Then**
   dòng status đỏ báo `Edit failed: <lỗi>` kèm đường dẫn bản sao tạm, và bản sao đó vẫn còn trên đĩa.

---

### User Story 2 - Xem/sửa file cục bộ (Priority: P2)

Người dùng chọn một file ở pane Local và bấm `e`; editor mở thẳng file đó. Khi thoát editor, file
manager quay lại, pane Local được nạp lại (kích thước, thời gian sửa cập nhật) và dòng status báo
`Edited <name>`.

**Vì sao ưu tiên này**: Hữu ích khi chuẩn bị file trước khi tải lên, nhưng người dùng vẫn có thể mở
terminal khác để làm việc này.

**Kiểm thử độc lập**: Bấm `e` trên một file Local, sửa, thoát; xác nhận nội dung file trên đĩa và
thông tin trong pane cập nhật.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** pane Local đang active và con trỏ ở một file thường, **When** người dùng bấm `e` rồi
   thoát editor, **Then** file manager hiện lại, pane Local được nạp lại và dòng status báo
   `Edited <name>`.

---

### User Story 3 - Báo lỗi rõ ràng (Priority: P2)

Khi không mở được editor, editor kết thúc bất thường, hoặc không tải được file, người dùng thấy lý
do ở dòng status thay vì thao tác im lặng thất bại.

**Vì sao ưu tiên này**: Không có thông báo, người dùng không biết bản sửa đã lên server hay chưa.

**Kiểm thử độc lập**: Cấu hình editor không tồn tại, hoặc một file remote không đọc được, bấm `e` và
đọc dòng status.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** editor không tìm thấy, **When** người dùng bấm `e`, **Then** dòng status đỏ báo
   `Edit failed: <lỗi>`; file remote không bị đổi.
2. **Given** editor thoát với mã khác 0, **When** file manager quay lại, **Then** dòng status đỏ báo
   `Edit failed: <lỗi>` và file remote không bị ghi lại.
3. **Given** file remote không đọc được (ví dụ thiếu quyền), **When** người dùng bấm `e`, **Then**
   editor không được mở và dòng status đỏ báo `Edit failed: <lỗi>`.

---

### Trường hợp biên (Edge Cases)

- Con trỏ ở thư mục hoặc hàng `../`: bấm `e` không làm gì.
- Ô filter đang mở: gõ `e` là text của filter, không mở editor.
- Đang có transfer chạy: `e` ở pane Remote báo `A transfer is already running`; ở pane Local vẫn mở
  editor bình thường.
- Editor ghi lại file mà không đổi nội dung (`:w` rồi `:q`): coi là không đổi, báo `No changes`.
- Pane rỗng (không có hàng nào được chọn): bấm `e` không làm gì.
- Bản sao tạm của file remote chỉ chủ sở hữu đọc/ghi được, trong một thư mục tạm riêng cho mỗi lần
  sửa.
- File remote bị người khác sửa trong lúc đang mở editor: bản sửa của người dùng ghi đè (không phát
  hiện xung đột).

## Yêu cầu (Requirements) *(bắt buộc)*

### Yêu cầu chức năng (Functional Requirements)

- **FR-001**: Bấm `e` khi focus ở bảng PHẢI mở file thường đang chọn ở pane active trong editor dòng
  lệnh; với thư mục, hàng `../` hoặc pane rỗng thì không làm gì.
- **FR-002**: Trong lúc editor chạy, giao diện file manager PHẢI nhường toàn bộ terminal cho editor
  và được khôi phục nguyên trạng (pane active, thư mục, filter, chế độ sắp xếp) khi editor thoát.
- **FR-003**: Với pane Local, editor PHẢI mở trực tiếp file gốc; khi editor thoát thành công, pane
  Local PHẢI được nạp lại và dòng status báo `Edited <name>` (xanh).
- **FR-004**: Với pane Remote, hệ thống PHẢI tải file xuống một bản sao tạm trong thư mục tạm riêng,
  bản sao chỉ chủ sở hữu đọc/ghi được (0600), rồi mở editor trên bản sao đó.
- **FR-005**: Khi editor thoát thành công và nội dung bản sao khác nội dung đã tải xuống (so sánh
  theo nội dung, không theo thời gian sửa), hệ thống
  PHẢI ghi đè file remote bằng nội dung mới, giữ nguyên quyền truy cập của file remote, và báo
  `Saved <name>` (xanh).
- **FR-006**: Khi nội dung không đổi, hệ thống PHẢI không ghi lại file remote và báo `No changes`.
- **FR-007**: Thư mục tạm PHẢI bị xoá sau mỗi lần sửa remote, trừ khi tải lên thất bại; khi đó bản
  sao được giữ lại và đường dẫn của nó PHẢI có trong thông báo lỗi.
- **FR-008**: Mọi lỗi (editor không tìm thấy, editor thoát mã khác 0, tải xuống hoặc tải lên thất
  bại) PHẢI hiện ở dòng status màu đỏ dạng `Edit failed: <lỗi>`; khi editor lỗi, file remote không
  bị ghi lại.
- **FR-009**: Sau mỗi lần sửa remote (thành công, không đổi hay lỗi), pane Remote PHẢI được nạp lại.
- **FR-010**: Dòng hints PHẢI có thêm `e Edit`.
- **FR-011**: Editor PHẢI được chọn theo thứ tự: trường `Editor:` trong form Sync & Settings
  (lưu ở `~/.lazyssh/settings.json`, khoá `editor`) nếu khác rỗng; ngược lại biến môi trường
  `EDITOR` nếu khác rỗng; ngược lại `vi`. Giá trị được tách theo khoảng trắng thành chương trình và
  tham số (ví dụ `nvim`, `vim -u NONE`, `code --wait`); đường dẫn file được thêm vào làm tham số cuối.
- **FR-012**: Với pane Remote, nếu một transfer đang chạy thì bấm `e` PHẢI báo đỏ
  `A transfer is already running` và không mở editor; trong lúc sửa remote, transfer mới PHẢI bị từ
  chối theo cùng cách. Pane Local không bị ràng buộc này.

### Thực thể chính (Key Entities)

- **Bản sao tạm**: file cục bộ chứa nội dung file remote trong lúc sửa; tồn tại trong một thư mục
  tạm riêng, chỉ chủ sở hữu truy cập được, bị xoá khi kết thúc (trừ khi tải lên thất bại).

## Tiêu chí thành công (Success Criteria) *(bắt buộc)*

### Kết quả đo được (Measurable Outcomes)

- **SC-001**: Người dùng sửa một file trên server với 1 phím trong file manager cộng thao tác lưu và
  thoát editor, không cần lệnh tải xuống/tải lên nào khác.
- **SC-002**: 100% lần thoát editor mà không đổi nội dung không ghi gì lên server.
- **SC-003**: Sau 100% lần sửa remote kết thúc bình thường, không còn bản sao tạm trên máy cục bộ.
- **SC-004**: Không bản sửa nào bị mất khi tải lên thất bại: người dùng luôn được chỉ tới bản sao
  còn giữ.
- **SC-005**: Không phím tắt hiện có nào của file manager đổi hành vi.

## Giả định (Assumptions)

- Editor là chương trình dòng lệnh chạy trong cùng terminal (mặc định `vi`); editor mở cửa sổ riêng
  phải được cấu hình để chờ tới khi đóng file thì mới thoát.
- Lớp service chứa luồng sửa remote (tải xuống, gọi editor, so sánh, tải lên, dọn dẹp) và việc chọn
  lệnh editor; giao diện chỉ nối phím, tạm dừng TUI và hiển thị kết quả.
- Ngoài phạm vi: cấu hình editor riêng trong lazyssh, phát hiện xung đột khi file remote bị sửa đồng thời, sửa nhiều file một lúc, giới
  hạn kích thước file, trình xem nội bộ trong TUI.
- File lớn chỉ làm lâu hơn bước tải xuống/tải lên; không có thanh tiến độ cho thao tác sửa.
- Phần nối phím và chạy editor thật được kiểm thủ công theo quickstart; luồng tải xuống, phát hiện
  thay đổi, tải lên, dọn dẹp và chọn editor được kiểm bằng unit test.
