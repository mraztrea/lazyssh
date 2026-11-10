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

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/rivo/tview"
)

// connectErrorText is the body of the connection-failed dialog; ssh output is shown verbatim.
func connectErrorText(alias string, se *domain.SSHError) string {
	text := alias + ": " + se.Reason
	for _, part := range []string{se.Hint, se.Output} {
		if part != "" {
			text += "\n\n" + part
		}
	}
	return tview.Escape(text)
}

// showConnectError explains a failed ssh/sftp connection instead of silently returning to the
// list. retry repeats the same connection (ssh, ssh with forwards, or sftp).
func (t *tui) showConnectError(server domain.Server, err error, retry func()) {
	var se *domain.SSHError
	if !errors.As(err, &se) {
		se = &domain.SSHError{Reason: err.Error()}
	}
	modal := tview.NewModal().
		SetText(connectErrorText(server.Alias, se)).
		AddButtons([]string{"Retry", "Edit server", "Close"}).
		SetDoneFunc(func(_ int, label string) {
			t.returnToMain()
			switch label {
			case "Retry":
				retry()
			case "Edit server":
				t.showServerEditForm(server)
			}
		}).
		SetFocus(2)
	modal.SetTitle(" Connection failed ")
	t.app.SetRoot(modal, true)
}
