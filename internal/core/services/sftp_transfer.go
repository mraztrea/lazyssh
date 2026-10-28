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
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

const copyBufferSize = 1 << 20

// TransferProgress reports the file being copied (relative to the transfer root), its bytes
// done/total, and its 1-based position among all files of the transfer.
type TransferProgress struct {
	File                 string
	Done, Total          int64
	FileIndex, FileCount int
}

type copyJob struct {
	src, dst, rel string
	mode          os.FileMode
	size          int64
}

// side abstracts the three operations a copy needs on either end (local disk or SFTP).
type side struct {
	open   func(string) (io.ReadCloser, error)
	create func(string) (io.WriteCloser, error)
	chmod  func(string, os.FileMode) error
}

func localSide() side {
	return side{
		open: func(p string) (io.ReadCloser, error) { return os.Open(filepath.Clean(p)) },
		create: func(p string) (io.WriteCloser, error) {
			return os.OpenFile(filepath.Clean(p), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // chmod to the source mode after copy
		},
		chmod: os.Chmod,
	}
}

func remoteSide(c *sftp.Client) side {
	return side{
		open: func(p string) (io.ReadCloser, error) { return c.Open(p) },
		create: func(p string) (io.WriteCloser, error) {
			return c.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		},
		chmod: c.Chmod,
	}
}

// UploadPath copies a local file or directory tree into remoteDir/<base(localPath)>.
func UploadPath(c *sftp.Client, localPath, remoteDir string, progress func(TransferProgress)) error {
	name := filepath.Base(localPath)
	dstRoot := path.Join(remoteDir, name)
	root, err := filepath.EvalSymlinks(localPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return runJobs([]copyJob{{src: root, dst: dstRoot, rel: name, mode: info.Mode(), size: info.Size()}},
			localSide(), remoteSide(c), progress)
	}

	var dirs []string
	var jobs []copyJob
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		dst := path.Join(dstRoot, rel)
		if d.IsDir() {
			dirs = append(dirs, dst)
			return nil
		}
		// Stat follows symlinks; broken links and special files are skipped.
		if fi, statErr := os.Stat(p); statErr == nil && fi.Mode().IsRegular() {
			jobs = append(jobs, copyJob{src: p, dst: dst, rel: path.Join(name, rel), mode: fi.Mode(), size: fi.Size()})
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, d := range dirs {
		if err := c.MkdirAll(d); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return runJobs(jobs, localSide(), remoteSide(c), progress)
}

// DownloadPath copies a remote file or directory tree into localDir/<base(remotePath)>.
func DownloadPath(c *sftp.Client, remotePath, localDir string, progress func(TransferProgress)) error {
	name := path.Base(remotePath)
	dstRoot := filepath.Join(localDir, name)
	// c.Walk uses Lstat, so resolve a symlinked root first.
	root, err := c.RealPath(remotePath)
	if err != nil {
		return err
	}
	info, err := c.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return runJobs([]copyJob{{src: root, dst: dstRoot, rel: name, mode: info.Mode(), size: info.Size()}},
			remoteSide(c), localSide(), progress)
	}

	var dirs []string
	var jobs []copyJob
	walker := c.Walk(root)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}
		p, fi := walker.Path(), walker.Stat()
		rel := strings.TrimPrefix(strings.TrimPrefix(p, root), "/")
		dst := filepath.Join(dstRoot, filepath.FromSlash(rel))
		if fi.IsDir() {
			dirs = append(dirs, dst)
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			if fi, err = c.Stat(p); err != nil {
				continue
			}
		}
		if !fi.Mode().IsRegular() {
			continue
		}
		jobs = append(jobs, copyJob{src: p, dst: dst, rel: path.Join(name, rel), mode: fi.Mode(), size: fi.Size()})
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o750); err != nil {
			return err
		}
	}
	return runJobs(jobs, remoteSide(c), localSide(), progress)
}

// RemoveRemote deletes a remote file, symlink or directory tree. Symlinks are never followed.
func RemoveRemote(c *sftp.Client, p string) error {
	info, err := c.Lstat(p)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return c.Remove(p)
	}
	entries, err := c.ReadDir(p)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := RemoveRemote(c, path.Join(p, e.Name())); err != nil {
			return err
		}
	}
	return c.RemoveDirectory(p)
}

func runJobs(jobs []copyJob, from, to side, progress func(TransferProgress)) error {
	buf := make([]byte, copyBufferSize)
	for i, j := range jobs {
		p := TransferProgress{File: j.rel, Total: j.size, FileIndex: i + 1, FileCount: len(jobs)}
		if err := copyOne(j, from, to, buf, p, progress); err != nil {
			return fmt.Errorf("%s: %w", j.rel, err)
		}
	}
	return nil
}

func copyOne(j copyJob, from, to side, buf []byte, p TransferProgress, progress func(TransferProgress)) error {
	src, err := from.open(j.src)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	dst, err := to.create(j.dst)
	if err != nil {
		return err
	}
	if progress != nil {
		progress(p)
	}
	// Wrapping both ends hides ReaderFrom/WriterTo so the 1 MiB buffer and the counter are used.
	_, err = io.CopyBuffer(&progressWriter{w: dst, p: p, fn: progress}, struct{ io.Reader }{src}, buf)
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err // partial file is left in place on purpose
	}
	_ = to.chmod(j.dst, j.mode.Perm()) // best effort
	return nil
}

type progressWriter struct {
	w  io.Writer
	p  TransferProgress
	fn func(TransferProgress)
}

func (pw *progressWriter) Write(b []byte) (int, error) {
	n, err := pw.w.Write(b)
	pw.p.Done += int64(n)
	if pw.fn != nil {
		pw.fn(pw.p)
	}
	return n, err
}
