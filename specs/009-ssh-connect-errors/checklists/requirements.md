# Checklist chất lượng đặc tả: Báo lỗi kết nối SSH/SFTP và xử lý host key đã đổi

**Mục đích**: Xác nhận đặc tả đầy đủ và đạt chất lượng trước khi lập kế hoạch
**Ngày tạo**: 2026-11-10
**Feature**: [spec.md](../spec.md)

## Chất lượng nội dung

- [x] Không có chi tiết triển khai (ngôn ngữ, framework, API)
- [x] Tập trung vào giá trị cho người dùng
- [x] Viết cho người đọc không chuyên kỹ thuật
- [x] Đủ các mục bắt buộc

## Độ đầy đủ của yêu cầu

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Yêu cầu kiểm thử được và không mơ hồ
- [x] Tiêu chí thành công đo được
- [x] Tiêu chí thành công không phụ thuộc công nghệ
- [x] Đã định nghĩa các kịch bản chấp nhận
- [x] Đã nêu các trường hợp biên
- [x] Phạm vi được giới hạn rõ
- [x] Đã nêu phụ thuộc và giả định

## Sẵn sàng cho tính năng

- [x] Mọi yêu cầu chức năng có tiêu chí chấp nhận rõ
- [x] Kịch bản người dùng phủ các luồng chính
- [x] Tính năng đáp ứng các kết quả đo được trong Success Criteria
- [x] Không lộ chi tiết triển khai vào đặc tả

## Ghi chú

- Đặc tả nhắc tới `known_hosts`, `ssh`, fingerprint, `[host]:port`: đây là khái niệm người dùng
  OpenSSH nhìn thấy trực tiếp, không phải chi tiết triển khai.
- Các lựa chọn mặc định (phạm vi SFTP, nút mặc định, cách phân biệt lỗi với thoát phiên) được ghi ở
  mục Clarifications sau bước `/speckit-clarify`.
