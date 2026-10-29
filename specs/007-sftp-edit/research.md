# Research: Xem/sửa file bằng editor trong trình quản lý file SFTP

Không có mục NEEDS CLARIFICATION trong Technical Context; các quyết định dưới đây chốt cách hiện thực.

## R1: Chọn editor

- **Decision**: `strings.Fields(os.Getenv("EDITOR"))`, rỗng thì `vi`; đường dẫn file là tham số
  cuối.
- **Rationale**: `EDITOR` là quy ước Unix chuẩn; tách theo khoảng trắng đủ cho `code --wait`,
  `nvim -u NONE`. `vi` có trên mọi Linux/macOS.
- **Alternatives considered**: chạy qua `sh -c "$EDITOR \"$1\""` (hỗ trợ trích dẫn nhưng phụ thuộc
  shell, không chạy trên Windows); thêm `VISUAL` (không ai yêu cầu); cấu hình trong lazyssh (YAGNI).
- **Hệ quả**: giá trị `EDITOR` có dấu nháy (ví dụ `sh -c '...'`) không được hiểu như shell; quickstart
  dùng một script nhỏ làm editor giả.

## R2: Phát hiện thay đổi

- **Decision**: SHA-256 (`crypto/sha256`) của bản sao trước và sau khi editor chạy.
- **Rationale**: editor có thể ghi lại cùng nội dung (`:w`) làm mtime đổi; hash nội dung tránh tải
  lên thừa. Đọc lại file cục bộ để hash là rẻ so với tải qua mạng.
- **Alternatives considered**: mtime + size (báo sai khi `:w` không đổi); giữ toàn bộ nội dung cũ
  trong bộ nhớ để so (tốn RAM với file lớn).

## R3: Ghi lại file remote

- **Decision**: `remoteSide(c).create` (`OpenFile(O_WRONLY|O_CREATE|O_TRUNC)`) + `io.Copy`, không
  chmod.
- **Rationale**: ghi đè tại chỗ giữ inode nên giữ mode/owner của file remote; tái dùng `side` có sẵn.
  `UploadPath` làm việc với thư mục đích, tiến độ và chmod theo mode cục bộ (0600 của bản sao) nên
  không phù hợp.
- **Alternatives considered**: ghi file tạm trên remote rồi `PosixRename` (nguyên tử hơn nhưng đổi
  inode, mất owner/mode nếu không chép lại; không ai yêu cầu).
- **Giới hạn**: tải lên lỗi giữa chừng có thể để lại file remote bị cắt cụt; bản sao cục bộ được giữ
  và đường dẫn có trong thông báo để người dùng tải lên lại.

## R4: Thư mục tạm

- **Decision**: `os.MkdirTemp("", "lazyssh-edit-")` (mode 0700), file bên trong tạo bằng
  `localSide().create` (0600), tên file là `path.Base(remotePath)`.
- **Rationale**: thư mục riêng cho mỗi lần sửa tránh trùng tên và giữ tên gốc để editor nhận dạng
  kiểu file. Test đặt `TMPDIR` về `t.TempDir()` để không ghi ra `/tmp` thật.
- **Alternatives considered**: `os.CreateTemp` với tên ngẫu nhiên (mất đuôi file); thư mục dưới
  `~/.lazyssh` (không cần lưu lâu dài).

## R5: Chạy editor trong TUI

- **Decision**: `app.Suspend(func(){ ... })` gọi đồng bộ trong handler phím (goroutine UI), editor
  kế thừa `os.Stdin/Stdout/Stderr`; với Remote, toàn bộ `EditRemote` chạy trong `Suspend`.
- **Rationale**: cùng pattern với `handleServerConnect` và `handleFileManager`. Không cần goroutine:
  UI không vẽ gì khi đang suspend.
- **Alternatives considered**: tải xuống/tải lên ngoài `Suspend` với tiến độ trên dòng status (phức
  tạp hơn, không ai yêu cầu).
- **Giới hạn**: khi sửa file Local trong lúc một transfer đang chạy, các cập nhật tiến độ xếp hàng
  trong hàng đợi của tview (100 phần tử); hàng đợi đầy thì transfer tạm dừng tới khi editor thoát.

## R6: Cờ `busy`

- **Decision**: Remote dùng `busy.CompareAndSwap(false, true)` như `startTransfer`, nhả sau khi
  `Suspend` trả về; Local không kiểm `busy`.
- **Rationale**: một phiên SFTP chỉ chạy một thao tác tải tại một thời điểm, giống transfer. Sửa
  file Local không dùng phiên SFTP.
