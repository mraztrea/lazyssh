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

package ui

import (
	"errors"
	"path"
	"testing"
	"time"

	"github.com/Adembc/lazyssh/internal/core/services"
)

func TestSortEntries_DirsFirstCaseInsensitive(t *testing.T) {
	e := []fileEntry{{Name: "b.txt"}, {Name: "A", IsDir: true}, {Name: "a.txt"}, {Name: "B", IsDir: true}}
	sortEntries(e)
	want := []string{"A", "B", "a.txt", "b.txt"}
	for i, w := range want {
		if e[i].Name != w {
			t.Fatalf("got %v, want %v", e, want)
		}
	}
	if !e[0].IsDir || !e[1].IsDir || e[2].IsDir {
		t.Fatalf("dirs not first: %v", e)
	}
}

func TestFormatSize(t *testing.T) {
	cases := map[int64]string{0: "0", 512: "512", 1234: "1.2K", 3_500_000: "3.3M", 1_200_000_000: "1.1G"}
	for n, want := range cases {
		if got := formatSize(n); got != want {
			t.Errorf("formatSize(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestFilePane_LoadRendersParentRow(t *testing.T) {
	fail := false
	list := func(string) ([]fileEntry, error) {
		if fail {
			return nil, errors.New("denied")
		}
		return []fileEntry{{Name: "f.txt", Size: 1234, ModTime: time.Now()}, {Name: "dir", IsDir: true}}, nil
	}
	p := newFilePane("Remote", list)
	p.join = path.Join
	if err := p.Load("/srv/app"); err != nil {
		t.Fatal(err)
	}
	if p.GetRowCount() != 3+1 { // header + ".." + 2 entries
		t.Fatalf("rows = %d", p.GetRowCount())
	}
	if got := p.GetCell(1, 0).Text; got != "../" {
		t.Fatalf("row 1 = %q", got)
	}
	if got := p.GetCell(2, 0).Text; got != "dir/" {
		t.Fatalf("row 2 = %q (dirs first)", got)
	}
	p.Select(1, 0)
	if e, ok := p.Selected(); !ok || e.Name != ".." || !e.IsDir {
		t.Fatalf("Selected on row 1 = %+v %v", e, ok)
	}
	p.Select(3, 0)
	if e, ok := p.Selected(); !ok || e.Name != "f.txt" {
		t.Fatalf("Selected on row 3 = %+v %v", e, ok)
	}
	if p.cwd != "/srv/app" || p.ParentDir() != "/srv" {
		t.Fatalf("cwd=%q parent=%q", p.cwd, p.ParentDir())
	}

	fail = true
	if err := p.Load("/nope"); err == nil {
		t.Fatal("expected error")
	}
	if p.cwd != "/srv/app" || p.GetRowCount() != 4 {
		t.Fatalf("failed Load changed the pane: cwd=%q rows=%d", p.cwd, p.GetRowCount())
	}

	fail = false
	if err := p.Load("/"); err != nil || p.ParentDir() != "/" {
		t.Fatalf("root parent = %q, err %v", p.ParentDir(), err)
	}
}

func TestProgressText(t *testing.T) {
	p := services.TransferProgress{File: "a/b/c.txt", Done: 2_100_000, Total: 4_200_000, FileIndex: 3, FileCount: 10}
	if got := progressText("Upload", p, true, time.Second); got != "Uploading c.txt  3/10 files  50%  2.1 MB/s" {
		t.Fatalf("dir: %q", got)
	}
	p = services.TransferProgress{File: "f", Done: 5, Total: 0, FileIndex: 1, FileCount: 1}
	if got := progressText("Download", p, false, 0); got != "Downloading f  100%  0.0 MB/s" {
		t.Fatalf("file: %q", got)
	}
}
