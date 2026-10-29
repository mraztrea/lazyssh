# Hướng dẫn build và cài đặt LazySSH

## 1. Build bộ cài

Yêu cầu: Go (theo `go.mod`). Không cần cài GoReleaser, lệnh dưới tự tải.

```bash
./scripts/build-installers.sh           # bản thử, version v0.0.0-next
./scripts/build-installers.sh v1.0.0    # bản có version (tự tạo git tag nếu chưa có)
```

Kết quả nằm trong thư mục `dist/` (khoảng 30 giây):

| Hệ điều hành | File | Loại |
|---|---|---|
| Debian / Ubuntu | `lazyssh_<ver>_linux_<arch>.deb` | bộ cài |
| Fedora / RHEL / openSUSE | `lazyssh_<ver>_linux_<arch>.rpm` | bộ cài |
| Arch / CachyOS / Manjaro | `lazyssh_<ver>_linux_<arch>.pkg.tar.zst` | bộ cài |
| Alpine | `lazyssh_<ver>_linux_<arch>.apk` | bộ cài |
| macOS | `lazyssh_Darwin_<arch>.tar.gz` | file nén |
| Windows | `lazyssh_Windows_<arch>.zip` | file nén |

`<arch>`: `amd64` (máy PC thông thường), `arm64`, `386`, `armv6`.

## 2. Cài đặt

### Debian / Ubuntu

```bash
sudo apt install ./dist/lazyssh_*_linux_amd64.deb
```

### Fedora / RHEL

```bash
sudo dnf install ./dist/lazyssh_*_linux_amd64.rpm
```

### Arch / CachyOS / Manjaro

```bash
sudo pacman -U ./dist/lazyssh_*_linux_amd64.pkg.tar.zst
```

### Alpine

```bash
sudo apk add --allow-untrusted ./dist/lazyssh_*_linux_amd64.apk
```

Các bộ cài Linux đặt file chạy tại `/usr/bin/lazyssh` và tự cài OpenSSH client nếu thiếu.

### macOS

```bash
tar -xzf lazyssh_Darwin_arm64.tar.gz      # Intel: lazyssh_Darwin_x86_64.tar.gz
sudo mv lazyssh /usr/local/bin/
xattr -d com.apple.quarantine /usr/local/bin/lazyssh   # nếu macOS chặn file
```

### Windows

1. Giải nén `lazyssh_Windows_x86_64.zip`
2. Chép `lazyssh.exe` vào một thư mục có trong `PATH` (ví dụ `C:\Tools`)
3. Cần OpenSSH Client: Settings → Apps → Optional features → OpenSSH Client

## 3. Sử dụng

```bash
lazyssh
```

LazySSH đọc và ghi danh sách server trong `~/.ssh/config`.

## 4. Gỡ cài đặt

| Hệ điều hành | Lệnh |
|---|---|
| Debian / Ubuntu | `sudo apt remove lazyssh` |
| Fedora / RHEL | `sudo dnf remove lazyssh` |
| Arch / CachyOS | `sudo pacman -R lazyssh` |
| Alpine | `sudo apk del lazyssh` |
| macOS | `sudo rm /usr/local/bin/lazyssh` |
| Windows | Xoá `lazyssh.exe` |

Gỡ cài đặt không xoá `~/.ssh/config` và `~/.lazyssh/` (metadata). Xoá tay nếu cần.
