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
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

// EditorCommand returns the editor argv: the lazyssh setting if set, else $EDITOR, else vi.
// Values split on spaces ("code --wait"); the file to edit is appended by the caller.
func EditorCommand(setting, env string) []string {
	for _, v := range []string{setting, env} {
		if f := strings.Fields(v); len(f) > 0 {
			return f
		}
	}
	return []string{"vi"}
}

// EditRemote downloads remotePath into a private temp dir (0700, file 0600), calls run on the copy
// and, if its content changed, overwrites the remote file in place so its mode is kept. The temp
// dir is removed unless the upload fails; then the error names the kept copy so the edit is not lost.
func EditRemote(c *sftp.Client, remotePath string, run func(localPath string) error) (changed bool, err error) {
	dir, err := os.MkdirTemp("", "lazyssh-edit-")
	if err != nil {
		return false, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(dir)
		}
	}()
	local := filepath.Join(dir, path.Base(remotePath)) // keeps the extension for syntax highlighting
	if err := copyFile(remoteSide(c), localSide(), remotePath, local); err != nil {
		return false, err
	}
	before, err := hashFile(local)
	if err != nil {
		return false, err
	}
	if err := run(local); err != nil {
		return false, err
	}
	after, err := hashFile(local)
	if err != nil || after == before {
		return false, err
	}
	if err := copyFile(localSide(), remoteSide(c), local, remotePath); err != nil {
		keep = true
		return true, fmt.Errorf("%w (edited copy kept at %s)", err, local)
	}
	return true, nil
}

// copyFile copies src to dst without touching dst's mode.
func copyFile(from, to side, src, dst string) error {
	r, err := from.open(src)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	w, err := to.create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	if cerr := w.Close(); err == nil {
		err = cerr // SFTP write errors may only surface on close
	}
	return err
}

func hashFile(p string) ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte
	f, err := os.Open(filepath.Clean(p))
	if err != nil {
		return sum, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return sum, err
	}
	copy(sum[:], h.Sum(nil))
	return sum, nil
}
