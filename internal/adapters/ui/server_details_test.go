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
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

func TestServerDetails_PasswordNeverRendered(t *testing.T) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	pw := base64.RawURLEncoding.EncodeToString(b)
	sd := NewServerDetails()
	sd.UpdateServer(domain.Server{Alias: "a", Aliases: []string{"a"}, Host: "h", Port: 22, Password: pw})
	text := sd.GetText(false)
	if !strings.Contains(text, "(password)") {
		t.Fatal("details pane lacks the (password) marker")
	}
	if strings.Contains(text, pw) {
		t.Fatal("details pane renders the password")
	}
}

func TestServerDetails_HomeDir(t *testing.T) {
	sd := NewServerDetails()
	sd.UpdateServer(domain.Server{Alias: "a", Aliases: []string{"a"}, Host: "h", Port: 22, HomeDir: "/var/www"})
	if text := sd.GetText(false); !strings.Contains(text, "Home Dir: [white]/var/www[-]") {
		t.Fatalf("details pane lacks the home dir:\n%s", text)
	}
	sd.UpdateServer(domain.Server{Alias: "a", Aliases: []string{"a"}, Host: "h", Port: 22})
	if text := sd.GetText(false); strings.Contains(text, "Home Dir") {
		t.Fatal("details pane shows an empty home dir")
	}
}
