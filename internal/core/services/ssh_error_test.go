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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

// Messages below are verbatim OpenSSH 10.6 output (research.md R3).
const hostKeyChangedUser = `@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
IT IS POSSIBLE THAT SOMEONE IS DOING SOMETHING NASTY!
Someone could be eavesdropping on you right now (man-in-the-middle attack)!
It is also possible that a host key has just been changed.
The fingerprint for the ED25519 key sent by the remote host is
SHA256:k3nqAxo38Pg0bgF4cy2RBqL7Pn9EY0P1U3K0st0ABls.
Please contact your system administrator.
Add correct host key in /home/u/.ssh/known_hosts to get rid of this message.
Offending ECDSA key in /home/u/.ssh/known_hosts:19
Host key for 103.142.24.161 has changed and you have requested strict checking.
Host key verification failed.
`

// Captured from a piped stderr while ssh had the tty in raw mode, hence the \r\n.
const hostKeyChangedPort = "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\r\n" +
	"@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @\r\n" +
	"@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\r\n" +
	"IT IS POSSIBLE THAT SOMEONE IS DOING SOMETHING NASTY!\r\n" +
	"Someone could be eavesdropping on you right now (man-in-the-middle attack)!\r\n" +
	"It is also possible that a host key has just been changed.\r\n" +
	"The fingerprint for the ED25519 key sent by the remote host is\n" +
	"SHA256:xIn/1NOmdxl8eKUNqYyS//xsbQM5ipxV/TFBjjNRgVY.\r\n" +
	"Please contact your system administrator.\r\n" +
	"Add correct host key in /tmp/lz009/known_hosts to get rid of this message.\r\n" +
	"Offending ED25519 key in /tmp/lz009/known_hosts:1\r\n" +
	"Host key for [127.0.0.1]:22422 has changed and you have requested strict checking.\r\n" +
	"Host key verification failed.\r\n"

const unprotectedKey = `@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@         WARNING: UNPROTECTED PRIVATE KEY FILE!          @
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
Permissions 0644 for '/tmp/lz009/id644' are too open.
It is required that your private key files are NOT accessible by others.
This private key will be ignored.
Load key "/tmp/lz009/id644": bad permissions
u@127.0.0.1: Permission denied (publickey,keyboard-interactive).
`

func TestClassifySSHError(t *testing.T) {
	tests := []struct {
		name, stderr, reason string
		hostKey              *domain.HostKeyChange
		output               string // substring Output must contain
	}{
		{
			name: "host key changed (user report)", stderr: hostKeyChangedUser, reason: "Host key changed",
			hostKey: &domain.HostKeyChange{
				Host: "103.142.24.161", File: "/home/u/.ssh/known_hosts", Line: 19,
				KeyType: "ED25519", Fingerprint: "SHA256:k3nqAxo38Pg0bgF4cy2RBqL7Pn9EY0P1U3K0st0ABls",
			},
		},
		{
			name: "host key changed, non-default port, raw-mode CRLF", stderr: hostKeyChangedPort, reason: "Host key changed",
			hostKey: &domain.HostKeyChange{
				Host: "[127.0.0.1]:22422", File: "/tmp/lz009/known_hosts", Line: 1,
				KeyType: "ED25519", Fingerprint: "SHA256:xIn/1NOmdxl8eKUNqYyS//xsbQM5ipxV/TFBjjNRgVY",
			},
		},
		{
			name:   "host key changed without offending line offers no removal",
			stderr: strings.Replace(hostKeyChangedUser, "Offending ECDSA key in /home/u/.ssh/known_hosts:19\n", "", 1),
			reason: "Host key changed",
		},
		{
			name:   "unknown host with strict checking",
			stderr: "No ED25519 host key is known for [127.0.0.1]:22422 and you have requested strict checking.\nHost key verification failed.\n",
			reason: "Host key not verified",
		},
		{name: "key permissions beat permission denied", stderr: unprotectedKey, reason: "Private key permissions too open"},
		{
			name:   "permission denied",
			stderr: "u@127.0.0.1: Permission denied (publickey,keyboard-interactive).\n",
			reason: "Authentication failed", output: "Permission denied (publickey,keyboard-interactive).",
		},
		{
			name:   "too many authentication failures",
			stderr: "Received disconnect from 10.0.0.5 port 22:2: Too many authentication failures\nDisconnected from 10.0.0.5 port 22\n",
			reason: "Authentication failed",
		},
		{name: "refused", stderr: "ssh: connect to host 127.0.0.1 port 22499: Connection refused\n", reason: "Connection refused"},
		{name: "timeout", stderr: "ssh: connect to host 10.255.255.1 port 22422: Connection timed out\n", reason: "Connection timed out"},
		{name: "dns", stderr: "ssh: Could not resolve hostname nonexistent.invalid: Name or service not known\n", reason: "Host name not found"},
		{name: "no route", stderr: "ssh: connect to host 10.1.2.3 port 22: No route to host\n", reason: "Host unreachable"},
		{name: "network unreachable", stderr: "ssh: connect to host 2001:db8::1 port 22: Network is unreachable\n", reason: "Host unreachable"},
		{
			name: "unrecognized keeps verbatim output", stderr: "command-line: line 0: Bad configuration option: foo\n",
			reason: "SSH connection failed", output: "command-line: line 0: Bad configuration option: foo",
		},
		{name: "empty stderr falls back", stderr: "", reason: "SSH connection failed", output: "ssh exited with status 255"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifySSHError(tc.stderr, "ssh exited with status 255")
			if got.Reason != tc.reason {
				t.Fatalf("Reason = %q, want %q", got.Reason, tc.reason)
			}
			if tc.reason != "SSH connection failed" && got.Hint == "" {
				t.Error("Hint is empty")
			}
			if (got.HostKey == nil) != (tc.hostKey == nil) || (tc.hostKey != nil && *got.HostKey != *tc.hostKey) {
				t.Errorf("HostKey = %+v, want %+v", got.HostKey, tc.hostKey)
			}
			if !strings.Contains(got.Output, tc.output) {
				t.Errorf("Output = %q, want it to contain %q", got.Output, tc.output)
			}
			if strings.Contains(got.Output, "\r") {
				t.Errorf("Output keeps \\r: %q", got.Output)
			}
		})
	}
}

func TestClassifySSHError_OutputKeepsLastEightLines(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&b, "line %d\r\n\r\n", i)
	}
	got := classifySSHError(b.String(), "").Output
	if want := "line 5\nline 6\nline 7\nline 8\nline 9\nline 10\nline 11\nline 12"; got != want {
		t.Fatalf("Output = %q, want %q", got, want)
	}
}

func TestStoredPasswordError(t *testing.T) {
	got := storedPasswordError(errors.New("master key missing"))
	if got.Reason != "Stored password unavailable" || got.Output != "master key missing" || got.Hint == "" {
		t.Fatalf("got %+v", got)
	}
}
