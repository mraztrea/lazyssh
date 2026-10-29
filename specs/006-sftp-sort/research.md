# Research: Sắp xếp file trong trình quản lý file SFTP

Không có mục NEEDS CLARIFICATION trong Technical Context; các quyết định dưới đây chốt cách hiện thực.

## R1: Có sắp theo thời gian tạo được không

- **Decision**: Không; chỉ Name và Modified.
- **Rationale**: SFTP v3 (bản OpenSSH `sftp-server` dùng, cũng là bản `github.com/pkg/sftp` hỗ trợ)
  chỉ trả `atime` và `mtime` trong `ATTRS`; `os.FileInfo` của Go chỉ có `ModTime()`. Lấy birth time
  ở pane Local cần syscall riêng từng OS (`statx` trên Linux) mà pane Remote vẫn không có, hai pane
  sẽ hành xử khác nhau.
- **Alternatives considered**: dùng `atime` thay thế (không phải thời gian tạo, dễ gây hiểu nhầm);
  SFTP v4+ có `createtime` nhưng OpenSSH không hỗ trợ.

## R2: Kiểu dữ liệu cho chế độ sort

- **Decision**: `type paneSort struct{ byTime, desc bool }` với `String()`.
- **Rationale**: giá trị rỗng là mặc định `Name ↑` nên không cần khởi tạo; đổi trường/chiều là phủ
  định một bool ngay tại chỗ gọi, không cần `toggleField()`/`reverse()` như `SortMode` (enum 4 giá
  trị phải liệt kê từng cặp).
- **Alternatives considered**: tái dùng `SortMode` của `sort.go` (nhãn `Alias`/`Last SSH`, quy tắc
  pinned và "never" không áp dụng); enum 4 giá trị riêng (thêm hai switch chỉ để đổi bool).

## R3: Sort ở đâu

- **Decision**: chuyển `sortEntries` từ `Load` sang đầu `render()`, sort `p.all` tại chỗ trước khi
  lọc.
- **Rationale**: `render()` là điểm chung của `Load` (đổi thư mục, refresh, `reload` sau thao tác
  file), `SetFilter` và `SetSort`, nên FR-007 đúng mà không sửa chỗ gọi nào. Sort lại khi gõ filter
  tốn O(n log n) trên danh sách đã sort sẵn; với 1.000 entry là không đáng kể.
- **Alternatives considered**: sort trong `Load` và `SetSort` riêng (hai chỗ gọi, dễ quên khi thêm
  đường nạp mới).

## R4: Tie-break và hướng

- **Decision**: thư mục trước ở mọi chế độ; theo Name thì so tên thường theo chiều; theo Modified thì
  so mtime theo chiều, bằng nhau thì tên thường tăng dần bất kể chiều; `sort.SliceStable` giữ thứ tự
  gốc cho tên trùng khi khác hoa thường.
- **Rationale**: đúng quyết định trong Clarifications; tie-break cố định làm thứ tự ổn định giữa các
  lần refresh.
- **Alternatives considered**: đảo cả tie-break khi ↓ (cách "đảo toàn bộ danh sách"); bị loại vì
  Clarifications chốt tên tăng dần.

## R5: Giữ con trỏ khi đổi chế độ

- **Decision**: `SetSort` nhớ số hàng, `render()` (đưa về `../`), rồi `Select` lại số hàng đó.
- **Rationale**: giống `reload` trong `sftp_view.go`; số hàng không đổi khi chỉ sort lại nên không
  cần kẹp.
- **Alternatives considered**: giữ con trỏ trên cùng entry (theo tên) — hành vi hay hơn khi entry
  nhảy xa, nhưng spec chỉ yêu cầu không nhảy về `../`; để dành nếu người dùng cần.

## R6: Định dạng thời gian

- **Decision**: `ModTime.Format("2006-01-02 15:04:05")`, không đổi múi giờ.
- **Rationale**: `os.FileInfo.ModTime()` (Local) và `pkg/sftp` (`time.Unix`) đều trả `time.Time` ở
  giờ local; format trực tiếp cho giờ local (FR-008). Cột rộng thêm 14 ký tự; cột Name có
  `SetExpansion(1)` nên tự co lại.
