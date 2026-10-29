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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"golang.org/x/crypto/ssh"
)

func TestValidateServer_PrivateKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	validPEM := string(pem.EncodeToMemory(block))
	encBlock, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	encryptedPEM := string(pem.EncodeToMemory(encBlock))
	base := domain.Server{Alias: "a", Host: "1.2.3.4"}

	cases := []struct {
		name, key string
		wantErr   bool
	}{
		{"empty ok", "", false},
		{"valid ed25519", validPEM, false},
		{"crlf accepted", strings.ReplaceAll(validPEM, "\n", "\r\n"), false},
		{"passphrase protected accepted", encryptedPEM, false},
		{"garbage", "not a key", true},
	}
	for _, c := range cases {
		s := base
		s.PrivateKey = c.key
		if err := validateServer(s); (err != nil) != c.wantErr {
			t.Errorf("%s: got %v", c.name, err)
		}
	}
}

func TestValidateServer_RejectsDotAliases(t *testing.T) {
	for _, a := range []string{".", ".."} {
		if validateServer(domain.Server{Alias: a, Host: "h"}) == nil {
			t.Errorf("alias %q accepted", a)
		}
	}
}

func TestNormalizePrivateKey(t *testing.T) {
	if got := normalizePrivateKey("a\r\nb\r\n\r\n"); got != "a\nb\n" {
		t.Fatalf("got %q", got)
	}
	if got := normalizePrivateKey("  \n"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateServer_PasswordNewlineRejected(t *testing.T) {
	base := domain.Server{Alias: "a", Host: "1.2.3.4"}
	for _, pw := range []string{"a\nb", "a\rb"} {
		s := base
		s.Password = pw
		if err := validateServer(s); err == nil || err.Error() != "password must be a single line" {
			t.Errorf("multi-line password: got %v", err)
		}
	}
	s := base
	s.Password = " spaced "
	if err := validateServer(s); err != nil {
		t.Errorf("spaced password rejected: %v", err)
	}
}

func TestParseSSHG_UserHostKeyAlias(t *testing.T) {
	out := "user alice\nhostname 10.0.0.5\nport 2222\nhostkeyalias web.example\n"
	cfg := parseSSHG(out)
	if cfg["port"] != "2222" {
		t.Fatalf("port = %q", cfg["port"])
	}
	if u, h := userHostOf(cfg); u != "alice" || h != "web.example" {
		t.Fatalf("got %s@%s, want hostkeyalias to win", u, h)
	}
	for _, alias := range []string{"none", ""} {
		cfg = parseSSHG("user alice\nhostname 10.0.0.5\nhostkeyalias " + alias + "\n")
		if u, h := userHostOf(cfg); u != "alice" || h != "10.0.0.5" {
			t.Fatalf("hostkeyalias %q: got %s@%s", alias, u, h)
		}
	}
}
