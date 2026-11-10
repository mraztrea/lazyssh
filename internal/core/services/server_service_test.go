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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"go.uber.org/zap"
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

func TestValidateServer_HomeDirSingleLine(t *testing.T) {
	base := domain.Server{Alias: "a", Host: "1.2.3.4"}
	for _, d := range []string{"a\nb", "a\rb"} {
		s := base
		s.HomeDir = d
		if err := validateServer(s); err == nil || err.Error() != "home dir must be a single line" {
			t.Errorf("multi-line home dir: got %v", err)
		}
	}
	s := base
	s.HomeDir = "/var/www"
	if err := validateServer(s); err != nil {
		t.Errorf("home dir rejected: %v", err)
	}
}

func TestSSHArgs(t *testing.T) {
	auth := []string{"-o", "A"}
	pre := []string{"-L", "1:2:3"}
	post := []string{"-N"}
	got := sshArgs("h", auth, pre, post, "")
	if want := []string{"-L", "1:2:3", "-o", "A", "-N", "h"}; !slices.Equal(got, want) {
		t.Errorf("no remote: got %q, want %q", got, want)
	}
	remote := `cd '/x'; exec "$SHELL" -l`
	got = sshArgs("h", auth, pre, post, remote)
	if want := []string{"-o", "RemoteCommand=none", "-t", "-L", "1:2:3", "-o", "A", "-N", "h", remote}; !slices.Equal(got, want) {
		t.Errorf("with remote: got %q, want %q", got, want)
	}
}

// fakeRepo implements only what SSH needs; any other call panics on the nil embedded interface.
type fakeRepo struct {
	ports.ServerRepository
	recorded []string
	pwErr    error
}

func (r *fakeRepo) GetPassword(string) (string, error) { return "", r.pwErr }

func (r *fakeRepo) RecordSSH(alias string) error {
	r.recorded = append(r.recorded, alias)
	return nil
}

// fakeSSH puts an `ssh` on PATH that prints $FAKE_SSH_ERR to stderr and exits $FAKE_SSH_CODE.
func fakeSSH(t *testing.T, code, stderr string) {
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$FAKE_SSH_ERR\" >&2\nexit \"$FAKE_SSH_CODE\"\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("FAKE_SSH_CODE", code)
	t.Setenv("FAKE_SSH_ERR", stderr)
}

func TestRunSSH_ExitStatus(t *testing.T) {
	tests := []struct {
		name, code string
		pwErr      error
		reason     string // "" = nil error
		recorded   bool
	}{
		{name: "ssh error", code: "255", reason: "Connection refused"},
		{name: "remote status", code: "3"},
		{name: "success", code: "0", recorded: true},
		{name: "stored password unreadable", code: "0", pwErr: errors.New("master key missing"), reason: "Stored password unavailable"},
	}
	for _, tc := range tests {
		for _, withArgs := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/withArgs=%v", tc.name, withArgs), func(t *testing.T) {
				fakeSSH(t, tc.code, "ssh: connect to host h port 22: Connection refused")
				repo := &fakeRepo{pwErr: tc.pwErr}
				svc := NewServerService(zap.NewNop().Sugar(), repo)
				var err error
				if withArgs {
					err = svc.SSHWithArgs(domain.Server{Alias: "a"}, []string{"-L", "1:h:2"})
				} else {
					err = svc.SSH(domain.Server{Alias: "a"})
				}
				var se *domain.SSHError
				switch {
				case tc.reason == "" && err != nil:
					t.Fatalf("err = %v, want nil", err)
				case tc.reason != "" && (!errors.As(err, &se) || se.Reason != tc.reason):
					t.Fatalf("err = %#v, want SSHError %q", err, tc.reason)
				}
				if got := len(repo.recorded) == 1; got != tc.recorded {
					t.Fatalf("recorded = %v, want %v", repo.recorded, tc.recorded)
				}
			})
		}
	}
}

func TestRemoveKnownHost(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not installed")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "k")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "known_hosts")
	content := "[127.0.0.1]:2222 " + string(pub) + "other " + string(pub)
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := NewServerService(zap.NewNop().Sugar(), &fakeRepo{})

	if err := svc.RemoveKnownHost("[127.0.0.1]:2222", file); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(file)
	if strings.Contains(string(got), "2222") || !strings.HasPrefix(string(got), "other ") {
		t.Fatalf("known_hosts = %q", got)
	}
	if old, _ := os.ReadFile(file + ".old"); string(old) != content {
		t.Fatalf("known_hosts.old = %q", old)
	}

	for _, bad := range [][2]string{{"", file}, {"other", ""}, {"other", filepath.Join(dir, "missing")}} {
		if err := svc.RemoveKnownHost(bad[0], bad[1]); err == nil {
			t.Errorf("RemoveKnownHost(%q, %q) = nil, want error", bad[0], bad[1])
		}
	}
}
