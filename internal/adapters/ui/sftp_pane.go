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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type fileEntry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// sortEntries puts directories first, then files, each case-insensitively by name.
func sortEntries(e []fileEntry) {
	sort.SliceStable(e, func(i, j int) bool {
		if e[i].IsDir != e[j].IsDir {
			return e[i].IsDir
		}
		return strings.ToLower(e[i].Name) < strings.ToLower(e[j].Name)
	})
}

// formatSize renders bytes as 0, 512, 1.2K, 3.3M, 1.1G (1024-based).
func formatSize(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10)
	}
	f := float64(n)
	for _, unit := range []string{"K", "M", "G", "T"} {
		f /= 1024
		if f < 1024 || unit == "T" {
			return fmt.Sprintf("%.1f%s", f, unit)
		}
	}
	return "" // unreachable
}

// entryFromInfo builds a row; symlinks are resolved with stat so links to directories can be entered.
func entryFromInfo(info os.FileInfo, stat func() (os.FileInfo, error)) fileEntry {
	if info.Mode()&os.ModeSymlink != 0 {
		if target, err := stat(); err == nil {
			info = target
		}
	}
	return fileEntry{Name: info.Name(), IsDir: info.IsDir(), Size: info.Size(), ModTime: info.ModTime()}
}

func localList(dir string) ([]fileEntry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]fileEntry, 0, len(des))
	for _, d := range des {
		info, err := d.Info()
		if err != nil {
			continue // vanished between readdir and lstat
		}
		name := d.Name()
		e := entryFromInfo(info, func() (os.FileInfo, error) { return os.Stat(filepath.Join(dir, name)) })
		e.Name = name
		out = append(out, e)
	}
	return out, nil
}

// filePane is one side of the file manager: a table of the entries in cwd.
type filePane struct {
	*tview.Table
	title   string
	suffix  string // appended to the border title, e.g. " (alias)"
	cwd     string
	list    func(path string) ([]fileEntry, error)
	join    func(elem ...string) string
	entries []fileEntry
}

func newFilePane(title string, list func(string) ([]fileEntry, error)) *filePane {
	p := &filePane{Table: tview.NewTable(), title: title, list: list, join: filepath.Join}
	p.SetBorder(true).SetTitleAlign(tview.AlignLeft)
	p.SetSelectable(true, false).SetFixed(1, 0)
	return p
}

// Load lists dir and renders it; on error the pane is left unchanged.
func (p *filePane) Load(dir string) error {
	entries, err := p.list(dir)
	if err != nil {
		return err
	}
	sortEntries(entries)
	p.entries, p.cwd = entries, dir

	p.Clear()
	header := func(col int, text string) {
		p.SetCell(0, col, tview.NewTableCell(text).SetSelectable(false).SetTextColor(tcell.Color245))
	}
	header(0, "Name")
	header(1, "Size")
	header(2, "Modified")
	p.SetCell(1, 0, tview.NewTableCell("../").SetExpansion(1).SetTextColor(tcell.Color111))
	for i, e := range entries {
		row := i + 2
		name, size, color := e.Name, formatSize(e.Size), tcell.Color252
		if e.IsDir {
			name, size, color = e.Name+"/", "", tcell.Color111
		}
		p.SetCell(row, 0, tview.NewTableCell(tview.Escape(name)).SetExpansion(1).SetTextColor(color))
		p.SetCell(row, 1, tview.NewTableCell(size).SetAlign(tview.AlignRight))
		p.SetCell(row, 2, tview.NewTableCell(" "+e.ModTime.Format("01-02")))
	}
	p.SetTitle(fmt.Sprintf(" %s: %s%s ", p.title, p.cwd, p.suffix))
	p.Select(1, 0)
	p.ScrollToBeginning()
	return nil
}

// Selected returns the entry under the cursor; the parent row is {Name: "..", IsDir: true}.
func (p *filePane) Selected() (fileEntry, bool) {
	row, _ := p.GetSelection()
	switch {
	case row == 1:
		return fileEntry{Name: "..", IsDir: true}, true
	case row >= 2 && row-2 < len(p.entries):
		return p.entries[row-2], true
	}
	return fileEntry{}, false
}

// ParentDir is cwd's parent; at the root it stays at the root.
func (p *filePane) ParentDir() string { return p.join(p.cwd, "..") }

// Path joins a name onto cwd.
func (p *filePane) Path(name string) string { return p.join(p.cwd, name) }
