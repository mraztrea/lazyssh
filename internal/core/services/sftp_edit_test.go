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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pkg/sftp"
)

func TestEditorCommand(t *testing.T) {
	for env, want := range map[string][]string{
		"":            {"vi"},
		"   ":         {"vi"},
		"nano":        {"nano"},
		"code --wait": {"code", "--wait"},
	} {
		if got := EditorCommand(env); !reflect.DeepEqual(got, want) {
			t.Errorf("EditorCommand(%q) = %q, want %q", env, got, want)
		}
	}
}

// editFixture points TMPDIR at a test dir and returns a client, a 0640 remote file and TMPDIR.
func editFixture(t *testing.T) (c *sftp.Client, remote, tmp string) {
	t.Helper()
	tmp = t.TempDir()
	t.Setenv("TMPDIR", tmp)
	remote = filepath.Join(t.TempDir(), "app.conf")
	writeFile(t, remote, []byte("old"), 0o640)
	return pipeClient(t), remote, tmp
}

func assertGone(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Stat(p); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s still exists: %v", p, err)
	}
}

func TestEditRemote_Changed(t *testing.T) {
	c, remote, _ := editFixture(t)
	var local string
	changed, err := EditRemote(c, remote, func(p string) error {
		local = p
		if filepath.Base(p) != "app.conf" {
			t.Errorf("temp copy name %q", filepath.Base(p))
		}
		if st, err := os.Stat(p); err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("temp copy mode %v %v", st.Mode(), err)
		}
		assertFile(t, p, []byte("old"))
		return os.WriteFile(p, []byte("x"), 0o600) // shorter: the remote must be truncated
	})
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	assertFile(t, remote, []byte("x"))
	if st, _ := os.Stat(remote); st.Mode().Perm() != 0o640 {
		t.Fatalf("remote mode changed: %v", st.Mode())
	}
	assertGone(t, filepath.Dir(local))
}

func TestEditRemote_Unchanged(t *testing.T) {
	c, remote, _ := editFixture(t)
	var local string
	changed, err := EditRemote(c, remote, func(p string) error {
		local = p
		return os.WriteFile(p, []byte("old"), 0o600) // saved without changes
	})
	if err != nil || changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	assertFile(t, remote, []byte("old"))
	assertGone(t, filepath.Dir(local))
}

func TestEditRemote_RunError(t *testing.T) {
	c, remote, _ := editFixture(t)
	boom := errors.New("boom")
	var local string
	changed, err := EditRemote(c, remote, func(p string) error {
		local = p
		_ = os.WriteFile(p, []byte("new"), 0o600)
		return boom
	})
	if !errors.Is(err, boom) || changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	assertFile(t, remote, []byte("old"))
	assertGone(t, filepath.Dir(local))
}

func TestEditRemote_DownloadError(t *testing.T) {
	c, remote, tmp := editFixture(t)
	called := false
	_, err := EditRemote(c, remote+".missing", func(string) error { called = true; return nil })
	if err == nil || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Fatalf("temp dir left behind: %v", left)
	}
}

func TestEditRemote_UploadErrorKeepsCopy(t *testing.T) {
	c, remote, _ := editFixture(t)
	var local string
	changed, err := EditRemote(c, remote, func(p string) error {
		local = p
		// A directory in place of the file makes the upload fail, even as root.
		if err := os.Remove(remote); err != nil {
			return err
		}
		if err := os.Mkdir(remote, 0o755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte("new"), 0o600)
	})
	if err == nil || !changed || !strings.Contains(err.Error(), local) {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	assertFile(t, local, []byte("new"))
}
