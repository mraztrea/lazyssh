# Research: Lọc file trong trình quản lý file SFTP

Không có mục NEEDS CLARIFICATION trong Technical Context; các quyết định dưới đây chốt cách hiện thực.

## R1: Giữ chỉ số `Selected()` đúng khi lọc

- **Decision**: `filePane.entries` trở thành danh sách *đã lọc*; danh sách đầy đủ nằm ở `all`.
- **Rationale**: `Selected()`, `selected()`, `transfer`, `rename`, `remove` đều dựa trên
  `entries[row-2]`; giữ nghĩa "hàng đang hiển thị" thì không phải sửa chỗ gọi nào (FR-008).
- **Alternatives considered**: lưu ánh xạ hàng → chỉ số gốc (thêm trạng thái, không lợi gì); ẩn hàng
  bằng chiều cao 0 (tview không hỗ trợ).

## R2: Khi nào xoá filter

- **Decision**: `Load(dir)` xoá filter khi `dir != p.cwd`; nạp lại cùng thư mục giữ filter.
- **Rationale**: mọi đường đổi thư mục (Enter, `h`, Backspace, `g`, `..`) và mọi đường nạp lại
  (`R`, sau mkdir/rename/delete/transfer qua `reload`) đều đi qua `Load`; phân biệt bằng thư mục là
  một dòng, không cần cờ ở từng chỗ gọi.
- **Alternatives considered**: tham số `keepFilter` cho `Load` (sửa mọi chỗ gọi); xoá filter trong
  `fm.open` (`goTo` tới cùng thư mục cũng xoá, khác `reload`). Hệ quả chấp nhận: `h` ở thư mục gốc
  (parent của `/` là `/`) giữ filter vì thư mục không đổi.

## R3: Ô nhập nằm ở đâu và nhận phím thế nào

- **Decision**: một `tview.InputField` thay `status` trong `layout` bằng `RemoveItem`/`AddItem`;
  `handleKey` (input capture của layout) trả nguyên phím khi ô filter có focus.
- **Rationale**: `Flex` chạy input capture của chính nó trước khi chuyển phím cho con đang focus, nên
  nếu không nhường thì `u`, `d`, `q`, Esc... vẫn chạy như phím tắt (vi phạm FR-003). Swap item giữ
  layout có một dòng đáy như cũ; pattern `SetChangedFunc` + `SetInputCapture` lấy từ `search_bar.go`.
- **Alternatives considered**: `tview.Pages` chồng hai dòng (thêm một primitive chỉ để đổi 1 dòng);
  modal như `prompt()` (che pane, không thấy kết quả lọc khi gõ).

## R4: Hiển thị filter trong title

- **Decision**: thêm `tview.Escape(" [/<chuỗi>]")` vào title.
- **Rationale**: title tview hiểu `[...]` là thẻ màu; escape giữ đúng ký tự người dùng gõ.
- **Alternatives considered**: không escape (chuỗi như `red]` làm vỡ title).

## R5: `SetText` kích hoạt changed func

- **Decision**: chấp nhận việc `startFilter` gọi `SetText(filter)` kích hoạt `SetFilter` cùng chuỗi
  (render lại, con trỏ về `../`); Esc dùng chính cơ chế này để xoá filter.
- **Rationale**: tview `InputField.SetText` luôn gọi changed func; render lại cùng danh sách là rẻ và
  nhất quán với FR-002 (con trỏ về `../` khi filter được áp dụng).
