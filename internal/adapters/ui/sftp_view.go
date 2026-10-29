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
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/Adembc/lazyssh/internal/core/services"
	"github.com/gdamore/tcell/v2"
	"github.com/pkg/sftp"
	"github.com/rivo/tview"
)

const (
	fileManagerHints = "[white]Tab[-] Switch  • [white]Enter[-] Open  • [white]h[-] Up  • [white]u[-] Upload  • [white]d[-] Download  • [white]n[-] Mkdir  • [white]r[-] Rename  • [white]D[-] Delete  • [white]R[-] Refresh  • [white]e[-] Edit  • [white]g[-] Go to  • [white]/[-] Filter  • [white]s[-] Sort  • [white]S[-] Reverse  • [white]q[-] Close"
	colorError       = "#FF6B6B"
	colorOK          = "#A0FFA0"
	progressInterval = 100 * time.Millisecond
)

type fileManager struct {
	t       *tui
	alias   string
	sess    ports.SFTPSession
	client  *sftp.Client
	editor  string // domain.Settings.Editor; empty falls back to $EDITOR then vi
	homeDir string // domain.Server.HomeDir; empty opens the session's starting dir

	local, remote, active *filePane
	status                *tview.TextView
	filterInput           *tview.InputField // replaces status while the filter is edited
	layout                *tview.Flex

	busy   atomic.Bool // one transfer at a time
	closed bool        // UI goroutine only
}

// handleFileManager opens an SFTP session for the selected server. Connecting runs with the TUI
// suspended so OpenSSH can prompt for a passphrase, password or host key on the terminal.
func (t *tui) handleFileManager() {
	server, ok := t.serverList.GetSelectedServer()
	if !ok {
		return
	}
	alias := server.Alias
	var sess ports.SFTPSession
	var err error
	t.app.Suspend(func() {
		fmt.Printf("Opening SFTP session to %s…\n", alias)
		sess, err = t.sftpService.Connect(alias)
	})
	if err != nil {
		t.showStatusTempColor("SFTP failed: "+err.Error(), colorError)
		return
	}
	t.showFileManager(server, sess)
}

func (t *tui) showFileManager(server domain.Server, sess ports.SFTPSession) {
	alias := server.Alias
	fm := &fileManager{t: t, alias: alias, sess: sess, client: sess.Client(), homeDir: server.HomeDir}
	if s, err := t.syncService.Settings(); err == nil {
		fm.editor = s.Editor
	}
	fm.local = newFilePane("Local", localList)
	fm.remote = newFilePane("Remote", fm.remoteList)
	fm.remote.join = path.Join
	fm.remote.suffix = " (" + alias + ")"
	for _, p := range []*filePane{fm.local, fm.remote} {
		p.SetFocusFunc(func() { fm.markActive(p) })
	}

	fm.status = tview.NewTextView().SetDynamicColors(true)
	fm.status.SetBackgroundColor(tcell.Color235)
	fm.status.SetText(fileManagerHints)

	fm.filterInput = tview.NewInputField().SetLabel("Filter: ").SetFieldBackgroundColor(tcell.Color235)
	fm.filterInput.SetBackgroundColor(tcell.Color235)
	fm.filterInput.SetChangedFunc(func(q string) { fm.active.SetFilter(q) })
	fm.filterInput.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyDown, tcell.KeyEnter:
			fm.endFilter(1)
		case tcell.KeyUp:
			fm.endFilter(-1)
		case tcell.KeyTab:
			fm.endFilter(0)
		case tcell.KeyEscape:
			fm.filterInput.SetText("") // the changed func shows the full list again
			fm.endFilter(0)
		default:
			return ev
		}
		return nil
	})

	panes := tview.NewFlex().
		AddItem(fm.local, 0, 1, true).
		AddItem(fm.remote, 0, 1, false)
	fm.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(panes, 0, 1, true).
		AddItem(fm.status, 1, 0, false)
	fm.layout.SetInputCapture(fm.handleKey)

	home, err := os.UserHomeDir()
	if err != nil {
		home = "/"
	}
	if err := fm.local.Load(home); err != nil {
		fm.fail(err.Error())
	}
	wd, err := fm.client.Getwd()
	if err != nil {
		wd = "/"
	}
	start := services.ResolveRemoteDir(fm.homeDir, wd)
	err = fm.remote.Load(start)
	if err != nil && start != wd {
		fm.fail("Home dir not found: " + fm.homeDir)
		err = fm.remote.Load(wd)
	}
	if err != nil {
		fm.fail(err.Error())
	}
	fm.active = fm.local
	fm.show()
}

// ponytail: remote listing runs on the UI goroutine; move it to a goroutine if slow links make the UI stutter.
func (fm *fileManager) remoteList(dir string) ([]fileEntry, error) {
	infos, err := fm.client.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]fileEntry, 0, len(infos))
	for _, info := range infos {
		name := info.Name()
		e := entryFromInfo(info, func() (os.FileInfo, error) { return fm.client.Stat(path.Join(dir, name)) })
		e.Name = name
		out = append(out, e)
	}
	return out, nil
}

func (fm *fileManager) handleKey(ev *tcell.EventKey) *tcell.EventKey {
	if fm.filterInput.HasFocus() {
		return ev // the layout sees keys first; while filtering they are text
	}
	up := func() { fm.open(fm.active, fm.active.ParentDir()) }
	cur := fm.active.sort
	var action func()
	if ev.Key() == tcell.KeyRune {
		action = map[rune]func(){
			'h': up,
			'u': func() { fm.transfer(true) },
			'd': func() { fm.transfer(false) },
			'n': fm.mkdir,
			'r': fm.rename,
			'D': fm.remove,
			'R': fm.refresh,
			'e': fm.edit,
			'g': fm.goTo,
			'/': fm.startFilter,
			's': func() { fm.sortBy(paneSort{!cur.byTime, cur.desc}) },
			'S': func() { fm.sortBy(paneSort{cur.byTime, !cur.desc}) },
			'q': fm.close,
		}[ev.Rune()]
	} else {
		action = map[tcell.Key]func(){
			tcell.KeyTab:        fm.switchPane,
			tcell.KeyBacktab:    fm.switchPane,
			tcell.KeyEnter:      fm.enter,
			tcell.KeyBackspace:  up,
			tcell.KeyBackspace2: up,
			tcell.KeyEscape:     fm.close,
		}[ev.Key()]
	}
	if action == nil {
		return ev // j/k, arrows, PgUp/PgDn, Home/End go to the table
	}
	action()
	return nil
}

// startFilter swaps the hint line for the filter input, prefilled with the active pane's filter.
func (fm *fileManager) startFilter() {
	fm.filterInput.SetText(fm.active.filter)
	fm.layout.RemoveItem(fm.status).AddItem(fm.filterInput, 1, 0, false)
	fm.t.app.SetFocus(fm.filterInput)
}

// endFilter restores the hint line and focuses the active pane, moving its cursor by d rows.
func (fm *fileManager) endFilter(d int) {
	fm.layout.RemoveItem(fm.filterInput).AddItem(fm.status, 1, 0, false)
	fm.t.app.SetFocus(fm.active)
	if d != 0 {
		fm.active.step(d)
	}
}

func (fm *fileManager) sortBy(s paneSort) {
	fm.active.SetSort(s)
	fm.info("Sort: " + s.String())
}

func (fm *fileManager) switchPane() {
	if fm.active == fm.local {
		fm.t.app.SetFocus(fm.remote)
	} else {
		fm.t.app.SetFocus(fm.local)
	}
}

func (fm *fileManager) show() {
	active := fm.active // SetRoot focuses the first pane, whose focus func would reset fm.active
	fm.t.app.SetRoot(fm.layout, true)
	fm.t.app.SetFocus(active)
}

func (fm *fileManager) markActive(p *filePane) {
	fm.active = p
	for _, q := range []*filePane{fm.local, fm.remote} {
		color := tview.Styles.BorderColor
		if q == p {
			color = tcell.Color75
		}
		q.SetBorderColor(color)
	}
}

func (fm *fileManager) info(msg string) {
	fm.status.SetText("[" + colorOK + "]" + tview.Escape(msg) + "[-]")
}

func (fm *fileManager) fail(msg string) {
	fm.status.SetText("[" + colorError + "]" + tview.Escape(msg) + "[-]")
}

func (fm *fileManager) isLocal(p *filePane) bool { return p == fm.local }

// selected returns the active pane's entry, ignoring the ".." row.
func (fm *fileManager) selected(p *filePane) (fileEntry, bool) {
	e, ok := p.Selected()
	return e, ok && e.Name != ".."
}

func (fm *fileManager) open(p *filePane, dir string) {
	if err := p.Load(dir); err != nil {
		fm.fail(err.Error())
		return
	}
	fm.status.SetText(fileManagerHints)
}

func (fm *fileManager) enter() {
	e, ok := fm.active.Selected()
	if !ok || !e.IsDir {
		return
	}
	if e.Name == ".." {
		fm.open(fm.active, fm.active.ParentDir())
		return
	}
	fm.open(fm.active, fm.active.Path(e.Name))
}

// reload re-lists a pane in place, keeping the cursor row where possible.
func (fm *fileManager) reload(p *filePane) error {
	row, _ := p.GetSelection()
	if err := p.Load(p.cwd); err != nil {
		return err
	}
	p.Select(min(max(row, 1), p.GetRowCount()-1), 0)
	return nil
}

func (fm *fileManager) refresh() {
	if fm.closed {
		return
	}
	for _, p := range []*filePane{fm.local, fm.remote} {
		if err := fm.reload(p); err != nil {
			fm.fail(err.Error())
			return
		}
	}
}

func (fm *fileManager) close() {
	if fm.closed {
		return
	}
	fm.closed = true
	err := fm.sess.Close()
	fm.t.returnToMain()
	if err != nil {
		fm.t.logger.Warnw("sftp close", "alias", fm.alias, "error", err)
	}
	fm.t.showStatusTemp("SFTP session to " + fm.alias + " closed")
}

// transfer uploads the local selection (upload) or downloads the remote selection, regardless of
// which pane is active.
func (fm *fileManager) transfer(upload bool) {
	src, dst, verb := fm.local, fm.remote, "Upload"
	if !upload {
		src, dst, verb = fm.remote, fm.local, "Download"
	}
	e, ok := fm.selected(src)
	if !ok {
		return
	}
	if fm.busy.Load() {
		fm.fail("A transfer is already running")
		return
	}
	srcPath, dstDir := src.Path(e.Name), dst.cwd
	var err error
	if upload {
		_, err = fm.client.Stat(path.Join(dstDir, e.Name))
	} else {
		_, err = os.Stat(filepath.Join(dstDir, e.Name))
	}
	start := func() { fm.startTransfer(verb, e, srcPath, dstDir) }
	if err == nil {
		fm.confirm(fmt.Sprintf("Overwrite %s?", e.Name), "Overwrite", start)
		return
	}
	start()
}

func (fm *fileManager) startTransfer(verb string, e fileEntry, srcPath, dstDir string) {
	if !fm.busy.CompareAndSwap(false, true) {
		fm.fail("A transfer is already running")
		return
	}
	fm.info(verb + "ing " + e.Name + "…")
	go func() {
		defer fm.busy.Store(false)
		var last, fileStart time.Time
		fileIndex := 0
		progress := func(p services.TransferProgress) {
			now := time.Now()
			if p.FileIndex != fileIndex {
				fileIndex, fileStart = p.FileIndex, now
			}
			if now.Sub(last) < progressInterval && p.Done < p.Total {
				return
			}
			last = now
			text := progressText(verb, p, e.IsDir, now.Sub(fileStart))
			fm.t.app.QueueUpdateDraw(func() {
				if !fm.closed {
					fm.status.SetText(tview.Escape(text))
				}
			})
		}
		var err error
		if verb == "Upload" {
			err = services.UploadPath(fm.client, srcPath, dstDir, progress)
		} else {
			err = services.DownloadPath(fm.client, srcPath, dstDir, progress)
		}
		if err != nil {
			fm.t.logger.Errorw("sftp transfer failed", "alias", fm.alias, "op", verb, "src", srcPath, "dst", dstDir, "error", err)
		} else {
			fm.t.logger.Infow("sftp transfer done", "alias", fm.alias, "op", verb, "src", srcPath, "dst", dstDir)
		}
		fm.t.app.QueueUpdateDraw(func() {
			if fm.closed {
				return
			}
			fm.refresh()
			if err != nil {
				fm.fail(verb + " failed: " + err.Error())
				return
			}
			fm.info(verb + "ed " + e.Name)
		})
	}()
}

// progressText renders e.g. "Uploading c.txt  3/10 files  45%  2.1 MB/s".
func progressText(verb string, p services.TransferProgress, isDir bool, elapsed time.Duration) string {
	pct := int64(100)
	if p.Total > 0 {
		pct = min(p.Done*100/p.Total, 100)
	}
	speed := 0.0
	if s := elapsed.Seconds(); s > 0 {
		speed = float64(p.Done) / s / 1e6
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%sing %s  ", verb, path.Base(p.File))
	if isDir {
		fmt.Fprintf(&b, "%d/%d files  ", p.FileIndex, p.FileCount)
	}
	fmt.Fprintf(&b, "%d%%  %.1f MB/s", pct, speed)
	return b.String()
}

// validName rejects names that would leave the current directory.
func validName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}

func (fm *fileManager) after(p *filePane, err error, msg string) {
	if err != nil {
		fm.fail(err.Error())
		return
	}
	if err := fm.reload(p); err != nil {
		fm.fail(err.Error())
		return
	}
	fm.info(msg)
}

// edit opens the selected file in $EDITOR (vi) with the TUI suspended. Remote files are edited
// through a temp copy that is uploaded back only if its content changed.
func (fm *fileManager) edit() {
	p := fm.active
	e, ok := fm.selected(p)
	if !ok || e.IsDir {
		return
	}
	target := p.Path(e.Name)
	var err error
	if fm.isLocal(p) {
		fm.t.app.Suspend(func() { err = fm.runEditor(target) })
		fm.edited(p, err, "Edited "+e.Name)
		return
	}
	if !fm.busy.CompareAndSwap(false, true) {
		fm.fail("A transfer is already running")
		return
	}
	var changed bool
	fm.t.app.Suspend(func() { changed, err = services.EditRemote(fm.client, target, fm.runEditor) })
	fm.busy.Store(false)
	msg := "No changes"
	if changed {
		msg = "Saved " + e.Name
	}
	fm.edited(p, err, msg)
}

func (fm *fileManager) edited(p *filePane, err error, msg string) {
	if err != nil {
		fm.t.logger.Errorw("sftp edit failed", "alias", fm.alias, "error", err)
		_ = fm.reload(p) // the file may have changed even though the edit failed
		fm.fail("Edit failed: " + err.Error())
		return
	}
	fm.after(p, nil, msg)
}

// runEditor runs the user's editor on file, handing it the terminal.
func (fm *fileManager) runEditor(file string) error {
	argv := append(services.EditorCommand(fm.editor, os.Getenv("EDITOR")), file)
	cmd := exec.Command(argv[0], argv[1:]...) // #nosec G204 -- the user's own $EDITOR
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func (fm *fileManager) mkdir() {
	p := fm.active
	fm.prompt("New directory: ", "", func(name string) {
		if !validName(name) {
			return
		}
		target := p.Path(name)
		var err error
		if fm.isLocal(p) {
			err = os.Mkdir(target, 0o750)
		} else {
			err = fm.client.Mkdir(target)
		}
		fm.after(p, err, "Created "+name)
	})
}

func (fm *fileManager) rename() {
	p := fm.active
	e, ok := fm.selected(p)
	if !ok {
		return
	}
	fm.prompt("Rename to: ", e.Name, func(name string) {
		if !validName(name) || name == e.Name {
			return
		}
		var err error
		if fm.isLocal(p) {
			err = os.Rename(p.Path(e.Name), p.Path(name))
		} else {
			err = fm.client.Rename(p.Path(e.Name), p.Path(name))
		}
		fm.after(p, err, "Renamed "+e.Name+" to "+name)
	})
}

func (fm *fileManager) remove() {
	p := fm.active
	e, ok := fm.selected(p)
	if !ok {
		return
	}
	target := p.Path(e.Name)
	fm.confirm(fmt.Sprintf("Delete %s?", e.Name), "Delete", func() {
		fm.info("Deleting " + e.Name + "…")
		go func() { // recursive remote deletes can take a while
			var err error
			if fm.isLocal(p) {
				err = os.RemoveAll(target)
			} else {
				err = services.RemoveRemote(fm.client, target)
			}
			if err != nil {
				fm.t.logger.Errorw("sftp delete failed", "alias", fm.alias, "path", target, "error", err)
			}
			fm.t.app.QueueUpdateDraw(func() {
				if !fm.closed {
					fm.after(p, err, "Deleted "+e.Name)
				}
			})
		}()
	})
}

func (fm *fileManager) goTo() {
	p := fm.active
	fm.prompt("Go to path: ", p.cwd, func(input string) {
		abs := path.IsAbs(input)
		if fm.isLocal(p) {
			abs = filepath.IsAbs(input)
		}
		target := p.join(input) // cleans
		if !abs {
			target = p.Path(input)
		}
		fm.open(p, target)
	})
}

// prompt shows a one-line input; Enter submits non-empty text, Esc cancels.
func (fm *fileManager) prompt(label, initial string, done func(string)) {
	input := tview.NewInputField().SetLabel(label).SetText(initial)
	input.SetBorder(true).SetTitle(" " + strings.TrimSuffix(label, ": ") + " ")
	input.SetDoneFunc(func(key tcell.Key) {
		text := strings.TrimSpace(input.GetText())
		fm.show()
		if key == tcell.KeyEnter && text != "" {
			done(text)
		}
	})
	fm.t.app.SetRoot(centered(input, 70, 3), true)
	fm.t.app.SetFocus(input)
}

func (fm *fileManager) confirm(text, okLabel string, yes func()) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"Cancel", okLabel}).
		SetDoneFunc(func(i int, _ string) {
			fm.show()
			if i == 1 {
				yes()
			}
		})
	fm.t.app.SetRoot(modal, true)
}

func centered(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 0, true).
			AddItem(nil, 0, 1, false), width, 0, true).
		AddItem(nil, 0, 1, false)
}
