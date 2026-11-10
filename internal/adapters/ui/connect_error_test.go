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
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

func TestConnectErrorText(t *testing.T) {
	got := connectErrorText("web", &domain.SSHError{
		Reason: "Connection refused",
		Hint:   "Check the port.",
		Output: "ssh: connect to host [::1] port 22: Connection refused",
	})
	for _, want := range []string{"web: Connection refused", "Check the port.", "ssh: connect to host [::1[] port 22"} {
		if !strings.Contains(got, want) {
			t.Errorf("text %q lacks %q", got, want)
		}
	}
}
