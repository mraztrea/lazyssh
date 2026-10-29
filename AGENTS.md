# Hướng dẫn cho agent

## Ngôn ngữ

- Toàn bộ phản hồi cho người dùng viết bằng **tiếng Việt**.
- Toàn bộ tài liệu tạo mới (spec, plan, tasks, docs, ghi chú) viết bằng **tiếng Việt**.
- Giữ nguyên tiếng Anh cho: mã nguồn, tên định danh, commit message, tên file, thuật ngữ kỹ thuật không có bản dịch thông dụng (ví dụ: `IdentityFile`, `SSH_ASKPASS`, AES-GCM).

## Quy trình

- Hiến pháp dự án: `.specify/memory/constitution.md` — tuân thủ trước mọi quy tắc khác.
- Tính năng mới đi qua spec-kit: `speckit-specify` → `speckit-clarify` → `speckit-plan` → `speckit-tasks` → `speckit-implement`.
