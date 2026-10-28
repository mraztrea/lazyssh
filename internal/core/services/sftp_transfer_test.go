// Copyright 2025.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
)

// pipeClient returns a client talking to an in-process SFTP server over the real filesystem.
func pipeClient(t *testing.T) *sftp.Client {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	server, err := sftp.NewServer(serverConn)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	client, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	return client
}

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil { // umask-proof
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s: content differs (%d vs %d bytes)", path, len(got), len(want))
	}
}

func TestUploadPath_FileWithProgress(t *testing.T) {
	c := pipeClient(t)
	src := filepath.Join(t.TempDir(), "big.bin")
	data := make([]byte, 3<<20)
	_, _ = rand.Read(data)
	writeFile(t, src, data, 0o640)
	dest := t.TempDir()

	var last TransferProgress
	calls := 0
	if err := UploadPath(c, src, dest, func(p TransferProgress) { last = p; calls++ }); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dest, "big.bin"), data)
	if calls < 2 || last.Done != last.Total || last.Total != 3<<20 || last.FileCount != 1 || last.FileIndex != 1 || last.File != "big.bin" {
		t.Fatalf("progress calls=%d last=%+v", calls, last)
	}
	st, err := os.Stat(filepath.Join(dest, "big.bin"))
	if err != nil || st.Mode().Perm() != 0o640 {
		t.Fatalf("mode not preserved: %v %v", st.Mode(), err)
	}
}

func TestUploadPath_DirectoryRecursive(t *testing.T) {
	c := pipeClient(t)
	root := filepath.Join(t.TempDir(), "a")
	writeFile(t, filepath.Join(root, "x.txt"), []byte("x"), 0o600)
	writeFile(t, filepath.Join(root, "b", "c.txt"), []byte("c"), 0o600)
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()

	var last TransferProgress
	if err := UploadPath(c, root, dest, func(p TransferProgress) { last = p }); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dest, "a", "x.txt"), []byte("x"))
	assertFile(t, filepath.Join(dest, "a", "b", "c.txt"), []byte("c"))
	if st, err := os.Stat(filepath.Join(dest, "a", "empty")); err != nil || !st.IsDir() {
		t.Fatalf("empty dir not recreated: %v", err)
	}
	if last.FileCount != 2 || last.FileIndex != 2 {
		t.Fatalf("last progress %+v", last)
	}
}

func TestDownloadPath_FileAndDirectory(t *testing.T) {
	c := pipeClient(t)
	remote := t.TempDir()
	writeFile(t, filepath.Join(remote, "f.txt"), []byte("file"), 0o644)
	writeFile(t, filepath.Join(remote, "tree", "sub", "g.txt"), []byte("g"), 0o600)
	writeFile(t, filepath.Join(remote, "tree", "h.txt"), []byte("h"), 0o600)
	local := t.TempDir()

	if err := DownloadPath(c, filepath.Join(remote, "f.txt"), local, nil); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(local, "f.txt"), []byte("file"))

	var last TransferProgress
	if err := DownloadPath(c, filepath.Join(remote, "tree"), local, func(p TransferProgress) { last = p }); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(local, "tree", "sub", "g.txt"), []byte("g"))
	assertFile(t, filepath.Join(local, "tree", "h.txt"), []byte("h"))
	if last.FileCount != 2 {
		t.Fatalf("last progress %+v", last)
	}
	if st, _ := os.Stat(filepath.Join(local, "tree", "h.txt")); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode not preserved: %v", st.Mode())
	}
}

func TestRemoveRemote_Recursive(t *testing.T) {
	c := pipeClient(t)
	base := t.TempDir()
	r := filepath.Join(base, "r")
	writeFile(t, filepath.Join(r, "s", "t.txt"), []byte("t"), 0o600)
	writeFile(t, filepath.Join(r, "u.txt"), []byte("u"), 0o600)
	// A symlink to a directory outside r must be removed, not followed.
	outside := filepath.Join(base, "keep")
	writeFile(t, filepath.Join(outside, "k.txt"), []byte("k"), 0o600)
	if err := os.Symlink(outside, filepath.Join(r, "link")); err != nil {
		t.Fatal(err)
	}

	if err := RemoveRemote(c, r); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(r); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("r still exists: %v", err)
	}
	assertFile(t, filepath.Join(outside, "k.txt"), []byte("k"))
}
