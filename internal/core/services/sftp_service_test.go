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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"go.uber.org/zap"
)

// helperCmd re-runs the test binary as a fake `ssh -s sftp` (see TestHelperProcess).
func helperCmd(mode string) func(alias string, authArgs []string) *exec.Cmd {
	return func(string, []string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
		cmd.Env = append(os.Environ(), "GO_WANT_SFTP_HELPER="+mode)
		return cmd
	}
}

// TestHelperProcess is not a real test: it serves SFTP on stdin/stdout, or fails like ssh does.
func TestHelperProcess(t *testing.T) {
	switch os.Getenv("GO_WANT_SFTP_HELPER") {
	case "":
		return
	case "fail":
		_, _ = os.Stderr.WriteString("ssh: connect to host any port 22: Connection refused\n")
		os.Exit(255)
	default:
		server, err := sftp.NewServer(struct {
			io.Reader
			io.WriteCloser
		}{os.Stdin, os.Stdout})
		if err != nil {
			os.Exit(2)
		}
		_ = server.Serve()
		os.Exit(0)
	}
}

func TestConnect_ListsDirectoryAndCloses(t *testing.T) {
	svc := newSFTPServiceWithCmd(zap.NewNop().Sugar(), noPassword, helperCmd("1"))
	sess, err := svc.Connect("any")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := sess.Client().ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal(err, entries)
	}
	if err := sess.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestConnect_ProcessExitReturnsError(t *testing.T) {
	svc := newSFTPServiceWithCmd(zap.NewNop().Sugar(), noPassword, helperCmd("fail"))
	done := make(chan error, 1)
	go func() {
		sess, err := svc.Connect("any")
		if sess != nil {
			_ = sess.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "Connection refused") {
			t.Fatalf("error lacks last stderr line: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Connect hung after ssh exited")
	}
}

func TestNewSFTPService_RequestsSubsystemAfterAlias(t *testing.T) {
	// OpenSSH's -s takes no argument: the host comes first, then the subsystem name.
	svc := NewSFTPService(zap.NewNop().Sugar(), noPassword).(*sftpService)
	got := strings.Join(svc.cmdFor("web-1", nil).Args, " ")
	if want := "ssh -s -- web-1 sftp"; got != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

func noPassword(string) (string, error) { return "", nil }

func TestSFTPCmd_PasswordServer(t *testing.T) {
	svc := NewSFTPService(zap.NewNop().Sugar(), noPassword).(*sftpService)
	got := strings.Join(svc.cmdFor("web-1", authOpts).Args, " ")
	want := "ssh -o PreferredAuthentications=password,keyboard-interactive -o NumberOfPasswordPrompts=1 -s -- web-1 sftp"
	if got != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
	if got := strings.Join(svc.cmdFor("web-1", nil).Args, " "); got != "ssh -s -- web-1 sftp" {
		t.Fatalf("args = %q", got)
	}
}
