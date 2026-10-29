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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/rivo/tview"
)

const pullConfirmText = "Pull will REPLACE ~/.ssh/config and ~/.lazyssh/metadata.json (backup is taken). Continue?"

var syncProviders = []string{"file", "gist"}

func (t *tui) handleSync() {
	s, err := t.syncService.Settings()
	if err != nil {
		t.showStatusTempColor("Sync settings: "+err.Error(), "#FF6B6B")
		return
	}
	t.showSyncForm(s)
}

func formatSyncTime(ts time.Time) string {
	if ts.IsZero() {
		return "never"
	}
	return ts.Local().Format("2006-01-02 15:04:05")
}

func (t *tui) showSyncForm(s domain.Settings) {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" Sync & Settings ").SetTitleAlign(tview.AlignCenter)

	providerIdx := 0
	for i, p := range syncProviders {
		if p == s.Sync.Provider {
			providerIdx = i
		}
	}
	form.AddDropDown("Provider:", syncProviders, providerIdx, nil)
	form.AddInputField("File path:", s.Sync.FilePath, 50, nil, nil)
	form.AddInputField("Gist ID:", s.Sync.GistID, 40, nil, nil)
	form.AddPasswordField("GitHub token:", s.Sync.GistToken, 40, '*', nil)
	form.AddPasswordField("Password:", "", 40, '*', nil)
	form.AddInputField("Editor:", s.Editor, 40, nil, nil) // for the file manager's `e` key; empty = $EDITOR, then vi

	field := func(label string) string {
		return form.GetFormItemByLabel(label).(*tview.InputField).GetText()
	}
	// current returns the form's settings; the password is deliberately not part of them.
	current := func() domain.Settings {
		next := s
		_, next.Sync.Provider = form.GetFormItemByLabel("Provider:").(*tview.DropDown).GetCurrentOption()
		next.Sync.FilePath = strings.TrimSpace(field("File path:"))
		next.Sync.GistID = strings.TrimSpace(field("Gist ID:"))
		next.Sync.GistToken = strings.TrimSpace(field("GitHub token:"))
		next.Editor = strings.TrimSpace(field("Editor:"))
		return next
	}
	save := func() (domain.Settings, bool) {
		next := current()
		if err := t.syncService.SaveSettings(next); err != nil {
			t.showStatusTempColor("Saving sync settings failed: "+err.Error(), "#FF6B6B")
			return next, false
		}
		return next, true
	}
	// run saves the form, leaves the screen and performs Push/Pull in the background.
	run := func(verb string, op func(context.Context, string) error, refresh bool) {
		next, ok := save()
		password := field("Password:")
		t.returnToMain()
		if !ok {
			return
		}
		provider := next.Sync.Provider
		t.showStatusTemp(verb + "ing…")
		go func() {
			err := op(context.Background(), password)
			t.app.QueueUpdateDraw(func() {
				if err != nil {
					t.showStatusTempColor(verb+" failed: "+err.Error(), "#FF6B6B")
					return
				}
				if refresh {
					t.refreshServerList()
				}
				t.showStatusTemp(fmt.Sprintf("%sed (%s)", verb, provider))
			})
		}()
	}

	footer := tview.NewTextView().SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("Last push: %s / Last pull: %s",
			formatSyncTime(s.Sync.LastPushAt), formatSyncTime(s.Sync.LastPullAt)))
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 0, 1, true).
		AddItem(footer, 1, 0, false)

	form.AddButton("Push", func() { run("Push", t.syncService.Push, false) })
	form.AddButton("Pull", func() {
		modal := tview.NewModal().
			SetText(pullConfirmText).
			AddButtons([]string{"Cancel", "Pull"}).
			SetDoneFunc(func(buttonIndex int, _ string) {
				if buttonIndex == 1 {
					run("Pull", t.syncService.Pull, true)
					return
				}
				t.app.SetRoot(layout, true)
				t.app.SetFocus(form)
			})
		t.app.SetRoot(modal, true)
	})
	form.AddButton("Save settings", func() {
		if _, ok := save(); ok {
			t.returnToMain()
			t.showStatusTemp("Sync settings saved")
		}
	})
	form.AddButton("Cancel", t.returnToMain)
	form.SetCancelFunc(t.returnToMain)

	t.app.SetRoot(layout, true)
	t.app.SetFocus(form)
}
