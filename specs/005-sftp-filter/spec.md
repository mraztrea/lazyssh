# Đặc tả tính năng: Lọc file trong trình quản lý file SFTP

**Feature Branch**: `005-sftp-filter`

**Ngày tạo**: 2026-10-29

**Trạng thái**: Draft

**Input**: Mô tả của người dùng: "Trong file manager SFTP (màn hình mở bằng phím `m`), thêm chế độ
lọc file cho pane đang active. Bấm `/` để vào chế độ filter: hiện một ô nhập ở dòng trạng thái (thay
cho dòng hints). Khi gõ, pane đang active chỉ hiển thị các entry có tên chứa chuỗi đã gõ (không phân
biệt hoa thường, substring). Hàng `../` luôn giữ nguyên. Lên/Xuống/Enter trong ô filter chuyển focus
về bảng của pane đó, giữ filter; Xuống/Enter đưa con trỏ tới hàng kế tiếp, Lên về hàng trước. Filter
đang áp dụng hiện trong title của pane (ví dụ ` Remote: /srv (alias) [/abc] `). Esc trong ô filter
xoá chuỗi, hiện lại toàn bộ danh sách, focus về bảng. Bấm `/` lần nữa khi filter đang áp dụng thì
mở lại ô nhập với chuỗi hiện tại để sửa."

**Xây dựng trên**: `specs/003-sftp-file-manager` (hai pane Local/Remote, dòng hints, phím tắt).

## Clarifications

### Session 2026-10-29

Trả lời không tương tác (answered non-interactively) dựa trên các quyết định đi kèm yêu cầu tính năng (pane áp dụng, đổi thư mục, phím tắt trong ô filter, không có entry khớp, hiển thị filter).

- Q: Filter áp dụng cho pane nào, và hai pane có chung một filter không? → A: Chỉ pane đang active lúc bấm `/`; mỗi pane giữ filter riêng, chuyển pane không xoá filter của pane kia (FR-001, FR-002).
- Q: Khi đổi thư mục (Enter vào thư mục, `h`, `g`, `..`) thì filter ra sao? → A: Filter của pane đó bị xoá, thư mục mới luôn hiện đầy đủ. Refresh (`R`) và nạp lại sau mkdir/rename/delete/transfer giữ filter (FR-007).
- Q: Các phím tắt khác (`u`, `d`, `n`, `r`, `D`, `q`...) có chạy khi đang ở ô filter không? → A: Không; ô filter nuốt mọi phím ký tự làm text, chỉ Lên/Xuống/Enter/Esc có nghĩa đặc biệt. Tab cũng rời ô nhập và focus về bảng, giữ filter (FR-003, FR-004).
- Q: Không entry nào khớp thì bảng hiển thị gì? → A: Bảng chỉ còn hàng `../` và con trỏ ở hàng đó (FR-002).
- Q: Người dùng biết filter đang áp dụng bằng cách nào sau khi rời ô nhập? → A: Title của pane có thêm ` [/<chuỗi>]` ở cuối; dòng hints có thêm `/ Filter` (FR-006, FR-009).

## Kịch bản người dùng & Kiểm thử (User Scenarios & Testing) *(bắt buộc)*

### User Story 1 - Lọc nhanh pane đang active theo tên (Priority: P1)

Người dùng đang ở pane Remote của một thư mục có hàng trăm file, bấm `/`, gõ `log`. Dòng hints được
thay bằng ô nhập; pane Remote chỉ còn hàng `../` và các entry có tên chứa `log` (bất kể hoa thường,
ví dụ `app.LOG`, `logs/`). Pane Local không đổi.

**Vì sao ưu tiên này**: Đây là khả năng cốt lõi người dùng yêu cầu; thiếu nó thì phải cuộn tay qua
thư mục lớn.

**Kiểm thử độc lập**: Nạp một pane với danh sách giả có tên hoa/thường lẫn lộn, đặt filter, xác nhận
số hàng, thứ tự và hàng `../` còn nguyên.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** pane Remote đang active, **When** người dùng bấm `/` và gõ `log`, **Then** pane Remote
   chỉ hiện `../` và các entry có tên chứa `log` không phân biệt hoa thường, theo thứ tự sắp xếp cũ.
2. **Given** ô filter đang mở, **When** người dùng gõ thêm hoặc xoá ký tự, **Then** danh sách cập
   nhật ngay theo chuỗi hiện tại.
3. **Given** không entry nào khớp, **When** filter được áp dụng, **Then** bảng chỉ còn hàng `../`
   và con trỏ ở hàng đó.
4. **Given** ô filter đang mở, **When** người dùng gõ `u`, `d`, `q` hay bất kỳ ký tự nào, **Then**
   ký tự đó vào ô nhập; không phím tắt nào của file manager chạy.

---

### User Story 2 - Rời ô filter để thao tác trên kết quả (Priority: P1)

Sau khi gõ filter, người dùng bấm Xuống (hoặc Enter) để focus về bảng và con trỏ nhảy tới entry đầu
tiên khớp; từ đó dùng các phím quen thuộc (`d` tải về, `r` đổi tên...). Title pane hiện
` Remote: /srv (alias) [/log] ` để nhớ rằng danh sách đang bị lọc.

**Vì sao ưu tiên này**: Lọc chỉ có ích khi thao tác được ngay trên kết quả.

**Kiểm thử độc lập**: Đặt filter, mô phỏng rời ô nhập bằng Xuống/Lên, xác nhận hàng được chọn,
entry `Selected()` đúng và title chứa `[/<chuỗi>]`.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** ô filter đang mở và con trỏ ở `../`, **When** người dùng bấm Xuống hoặc Enter,
   **Then** focus về bảng của pane đó, filter giữ nguyên và con trỏ ở entry khớp đầu tiên.
2. **Given** ô filter đang mở, **When** người dùng bấm Lên, **Then** focus về bảng và con trỏ lùi
   một hàng (không vượt lên trên `../`).
3. **Given** ô filter đang mở, **When** người dùng bấm Tab, **Then** focus về bảng, filter giữ
   nguyên, con trỏ không đổi.
4. **Given** filter `log` đang áp dụng và focus ở bảng, **When** người dùng bấm `d` trên một entry,
   **Then** đúng entry đang hiển thị dưới con trỏ được tải về.
5. **Given** filter đang áp dụng, **When** người dùng nhìn pane, **Then** title kết thúc bằng
   ` [/<chuỗi>] ` và dòng hints có `/ Filter`.

---

### User Story 3 - Huỷ hoặc sửa filter (Priority: P2)

Người dùng bấm Esc trong ô filter để bỏ lọc và thấy lại toàn bộ thư mục; hoặc bấm `/` lần nữa khi
filter đang áp dụng để sửa chuỗi cũ.

**Vì sao ưu tiên này**: Cần để thoát khỏi trạng thái lọc, nhưng Story 1-2 đã dùng được mà không cần
sửa chuỗi.

**Kiểm thử độc lập**: Đặt filter, xoá bằng chuỗi rỗng, xác nhận toàn bộ entry trở lại và title
không còn `[/...]`.

**Kịch bản chấp nhận (Acceptance Scenarios)**:

1. **Given** ô filter đang mở với chuỗi `log`, **When** người dùng bấm Esc, **Then** chuỗi bị xoá,
   pane hiện lại toàn bộ danh sách, dòng hints trở lại, focus về bảng.
2. **Given** filter `log` đang áp dụng và focus ở bảng, **When** người dùng bấm `/`, **Then** ô
   nhập mở lại với sẵn `log` để sửa.

---

### Trường hợp biên (Edge Cases)

- Đổi thư mục (Enter vào thư mục, `h`, Backspace, `g`, hàng `../`): filter của pane đó bị xoá, thư
  mục mới hiện đầy đủ.
- Refresh (`R`) và nạp lại sau mkdir/rename/delete/transfer: filter được giữ, danh sách mới được
  lọc lại theo chuỗi cũ.
- Mỗi pane giữ filter riêng: lọc Remote không ảnh hưởng Local, chuyển pane bằng Tab không xoá filter.
- Chuỗi filter chứa `[`/`]`: title vẫn hiển thị đúng ký tự, không bị hiểu thành mã màu.
- Esc trong ô filter chỉ xoá filter, không đóng file manager; Esc ở bảng vẫn đóng như cũ.

## Yêu cầu (Requirements) *(bắt buộc)*

### Yêu cầu chức năng (Functional Requirements)

- **FR-001**: Bấm `/` khi focus ở một pane PHẢI thay dòng hints bằng ô nhập `Filter: ` với chuỗi
  filter hiện tại của pane đang active và focus vào ô nhập.
- **FR-002**: Mỗi thay đổi trong ô nhập PHẢI lọc lại pane đang active: chỉ giữ entry có tên chứa
  chuỗi (substring, không phân biệt hoa thường), giữ thứ tự sắp xếp; hàng `../` luôn hiện; chuỗi
  rỗng hiện đầy đủ. Con trỏ về hàng `../`.
- **FR-003**: Trong ô nhập, mọi phím ký tự PHẢI là text; chỉ Lên, Xuống, Enter, Tab, Esc có nghĩa
  đặc biệt.
- **FR-004**: Xuống/Enter PHẢI đóng ô nhập, giữ filter, focus về bảng của pane đó và đưa con trỏ
  xuống một hàng (không vượt hàng cuối); Lên tương tự nhưng lùi một hàng (không vượt lên trên
  `../`); Tab đóng ô nhập, giữ filter, không di chuyển con trỏ.
- **FR-005**: Esc trong ô nhập PHẢI xoá chuỗi, hiện lại toàn bộ danh sách, đóng ô nhập và focus về
  bảng.
- **FR-006**: Khi pane có filter khác rỗng, title PHẢI có thêm ` [/<chuỗi>]` ngay trước khoảng trắng
  cuối; chuỗi được escape để không bị hiểu là mã màu.
- **FR-007**: Pane nạp một thư mục khác thư mục hiện tại PHẢI xoá filter của pane đó; nạp lại cùng
  thư mục (refresh, sau thao tác file) PHẢI giữ filter.
- **FR-008**: Mọi thao tác dùng entry dưới con trỏ (Enter, `u`, `d`, `r`, `D`) PHẢI tác động lên
  entry đang hiển thị ở hàng đó trong danh sách đã lọc.
- **FR-009**: Dòng hints PHẢI có thêm `/ Filter`.

### Thực thể chính (Key Entities)

- **Filter của pane**: chuỗi người dùng gõ, gắn với một pane (Local hoặc Remote), tồn tại tới khi bị
  xoá bằng Esc, chuỗi rỗng hoặc đổi thư mục. Không lưu xuống đĩa.

## Tiêu chí thành công (Success Criteria) *(bắt buộc)*

### Kết quả đo được (Measurable Outcomes)

- **SC-001**: Từ lúc mở file manager, người dùng tới được một file đã biết tên trong thư mục 500
  entry với tối đa `/` + tên + 1 phím điều hướng.
- **SC-002**: Danh sách cập nhật ngay sau mỗi phím gõ, không có độ trễ nhận thấy được với thư mục
  1.000 entry.
- **SC-003**: 100% thao tác trên entry khi đang lọc tác động đúng entry hiển thị dưới con trỏ (kiểm
  bằng test).
- **SC-004**: Không phím tắt hiện có nào của file manager đổi hành vi khi không dùng filter.

## Giả định (Assumptions)

- Lọc chỉ dựa trên danh sách đã nạp; không truy vấn lại server khi gõ.
- Không hỗ trợ glob, regex, lọc đệ quy hay lưu filter giữa các phiên.
- Phần nối phím tcell (focus, swap ô nhập/hints) được kiểm thủ công theo quickstart; logic lọc, title
  và di chuyển con trỏ được kiểm bằng unit test.
