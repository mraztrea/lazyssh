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
	"strings"
	"testing"
	"time"

	"github.com/Adembc/lazyssh/internal/core/services"
)

func TestSortEntries_DirsFirstCaseInsensitive(t *testing.T) {
	e := []fileEntry{{Name: "b.txt"}, {Name: "A", IsDir: true}, {Name: "a.txt"}, {Name: "B", IsDir: true}}
	sortEntries(e, paneSort{})
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

func TestFilterEntries(t *testing.T) {
	e := []fileEntry{{Name: "Logs", IsDir: true}, {Name: "app.log"}, {Name: "readme.md"}}
	if got := filterEntries(e, ""); len(got) != 3 {
		t.Fatalf("empty query = %v", got)
	}
	got := filterEntries(e, "LOG")
	if len(got) != 2 || got[0].Name != "Logs" || got[1].Name != "app.log" {
		t.Fatalf("LOG = %v", got)
	}
	if got := filterEntries(e, "zzz"); len(got) != 0 {
		t.Fatalf("zzz = %v", got)
	}
}

func filterTestPane(t *testing.T) *filePane {
	t.Helper()
	list := func(string) ([]fileEntry, error) {
		return []fileEntry{{Name: "logs", IsDir: true}, {Name: "app.LOG"}, {Name: "readme.md"}}, nil
	}
	p := newFilePane("Remote", list)
	p.join = path.Join
	if err := p.Load("/srv"); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFilePane_SetFilter(t *testing.T) {
	p := filterTestPane(t)
	p.Select(3, 0)
	p.SetFilter("log")
	if p.GetRowCount() != 4 || p.GetCell(1, 0).Text != "../" {
		t.Fatalf("rows = %d, row 1 = %q", p.GetRowCount(), p.GetCell(1, 0).Text)
	}
	if row, _ := p.GetSelection(); row != 1 {
		t.Fatalf("selected row = %d, want 1", row)
	}
	p.Select(3, 0)
	if e, ok := p.Selected(); !ok || e.Name != "app.LOG" {
		t.Fatalf("Selected = %+v %v", e, ok)
	}
	if !strings.Contains(p.GetTitle(), "[/log") {
		t.Fatalf("title = %q", p.GetTitle())
	}
	p.SetFilter("zzz")
	if p.GetRowCount() != 2 {
		t.Fatalf("no match rows = %d", p.GetRowCount())
	}
	p.SetFilter("")
	if p.GetRowCount() != 5 || strings.Contains(p.GetTitle(), "[/") {
		t.Fatalf("cleared: rows = %d title = %q", p.GetRowCount(), p.GetTitle())
	}
}

func TestFilePane_LoadClearsFilterOnDirChange(t *testing.T) {
	p := filterTestPane(t)
	p.SetFilter("log")
	if err := p.Load("/srv"); err != nil || p.filter != "log" || p.GetRowCount() != 4 {
		t.Fatalf("same dir: filter=%q rows=%d err=%v", p.filter, p.GetRowCount(), err)
	}
	if err := p.Load("/srv/logs"); err != nil || p.filter != "" || p.GetRowCount() != 5 {
		t.Fatalf("new dir: filter=%q rows=%d err=%v", p.filter, p.GetRowCount(), err)
	}
}

func TestFilePane_Step(t *testing.T) {
	p := filterTestPane(t)
	p.SetFilter("log")
	for _, c := range []struct{ d, want int }{{1, 2}, {1, 3}, {1, 3}, {-5, 1}} {
		p.step(c.d)
		if row, _ := p.GetSelection(); row != c.want {
			t.Fatalf("step(%d) row = %d, want %d", c.d, row, c.want)
		}
	}
	p.SetFilter("zzz")
	p.step(1)
	if row, _ := p.GetSelection(); row != 1 {
		t.Fatalf("no match step row = %d", row)
	}
}

func TestFilePane_ModifiedColumn(t *testing.T) {
	list := func(string) ([]fileEntry, error) {
		return []fileEntry{{Name: "f.txt", ModTime: time.Date(2026, 10, 29, 14, 5, 9, 0, time.Local)}}, nil
	}
	p := newFilePane("Remote", list)
	if err := p.Load("/srv"); err != nil {
		t.Fatal(err)
	}
	if got := p.GetCell(2, 2).Text; got != " 2026-10-29 14:05:09" {
		t.Fatalf("modified = %q", got)
	}
}

func TestPaneSort_String(t *testing.T) {
	cases := map[paneSort]string{{}: "Name ↑", {desc: true}: "Name ↓", {byTime: true}: "Modified ↑", {true, true}: "Modified ↓"}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("%+v = %q, want %q", s, got, want)
		}
	}
}

// sortFixture: dirs d1 (old) and D2 (new); files b (t1), a and C (t2), e (t3).
func sortFixture() []fileEntry {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2, t3 := t1.Add(time.Hour), t1.Add(2*time.Hour)
	return []fileEntry{
		{Name: "e", ModTime: t3}, {Name: "D2", IsDir: true, ModTime: t3}, {Name: "C", ModTime: t2},
		{Name: "b", ModTime: t1}, {Name: "d1", IsDir: true, ModTime: t1}, {Name: "a", ModTime: t2},
	}
}

func names(e []fileEntry) string {
	var s []string
	for _, x := range e {
		s = append(s, x.Name)
	}
	return strings.Join(s, ",")
}

func TestSortEntries_Modes(t *testing.T) {
	cases := map[paneSort]string{
		{}:                         "d1,D2,a,b,C,e",
		{desc: true}:               "D2,d1,e,C,b,a",
		{byTime: true}:             "d1,D2,b,a,C,e",
		{byTime: true, desc: true}: "D2,d1,e,a,C,b",
	}
	for s, want := range cases {
		e := sortFixture()
		sortEntries(e, s)
		if got := names(e); got != want {
			t.Errorf("%s: got %s, want %s", s, got, want)
		}
	}
}

func rowNames(p *filePane) string {
	var s []string
	for r := 2; r < p.GetRowCount(); r++ {
		s = append(s, strings.TrimSuffix(p.GetCell(r, 0).Text, "/"))
	}
	return strings.Join(s, ",")
}

func sortTestPane(t *testing.T) *filePane {
	t.Helper()
	p := newFilePane("Remote", func(string) ([]fileEntry, error) { return sortFixture(), nil })
	p.join = path.Join
	if err := p.Load("/srv"); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFilePane_SetSort(t *testing.T) {
	p := sortTestPane(t)
	if !strings.Contains(p.GetTitle(), "[Name ↑") {
		t.Fatalf("default title = %q", p.GetTitle())
	}
	p.Select(4, 0)
	p.SetSort(paneSort{byTime: true, desc: true})
	if got := rowNames(p); got != "D2,d1,e,a,C,b" {
		t.Fatalf("rows = %s", got)
	}
	if row, _ := p.GetSelection(); row != 4 {
		t.Fatalf("selected row = %d, want 4", row)
	}
	if !strings.Contains(p.GetTitle(), "[Modified ↓") {
		t.Fatalf("title = %q", p.GetTitle())
	}
	p.SetFilter("c")
	if got := rowNames(p); got != "C" || !strings.Contains(p.GetTitle(), "[Modified ↓") {
		t.Fatalf("filtered rows = %s title = %q", got, p.GetTitle())
	}
}

func TestFilePane_LoadKeepsSort(t *testing.T) {
	p := sortTestPane(t)
	s := paneSort{byTime: true, desc: true}
	p.SetSort(s)
	if err := p.Load("/srv/other"); err != nil || p.sort != s {
		t.Fatalf("sort = %+v err = %v", p.sort, err)
	}
	if got := rowNames(p); got != "D2,d1,e,a,C,b" {
		t.Fatalf("rows = %s", got)
	}
}
