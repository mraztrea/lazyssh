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
	"regexp"
	"strconv"
	"strings"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

const outputLines = 8

// sshFailures maps OpenSSH's English messages to a cause, checked in order: the first match wins.
var sshFailures = []struct {
	match        []string
	reason, hint string
}{
	{
		[]string{"REMOTE HOST IDENTIFICATION HAS CHANGED"},
		"Host key changed",
		"This may be a man-in-the-middle attack, or the server was reinstalled. Verify the new fingerprint with the server's administrator, then remove the old key from known_hosts.",
	},
	{
		[]string{"Host key verification failed"},
		"Host key not verified",
		"The server's host key was not accepted. Retry and compare the fingerprint ssh shows before answering yes.",
	},
	// Before "Permission denied": an ignored key usually ends in a denied login too.
	{
		[]string{"UNPROTECTED PRIVATE KEY FILE", ": bad permissions"},
		"Private key permissions too open",
		"ssh ignores keys others can read. Run: chmod 600 <key file>.",
	},
	{
		[]string{"Permission denied (", "Too many authentication failures"},
		"Authentication failed",
		"Check the user, identity file or stored password (Edit server), and that your key is in the server's authorized_keys.",
	},
	{
		[]string{"Could not resolve hostname"},
		"Host name not found",
		"Check the Host/IP of this server (Edit server) or your DNS / VPN.",
	},
	{
		[]string{"Connection refused"},
		"Connection refused",
		"The host answered but nothing accepts SSH on that port. Check the port and that sshd is running.",
	},
	{
		[]string{"timed out"},
		"Connection timed out",
		"The host did not answer. Check the address, VPN/firewall, and that the server is up.",
	},
	{
		[]string{"No route to host", "Network is unreachable"},
		"Host unreachable",
		"Check your network connection, VPN, or the server address.",
	},
}

var (
	reChangedHost = regexp.MustCompile(`(?i)host key for (\S+) has changed`)
	reOffending   = regexp.MustCompile(`(?m)^Offending \S+ key in (.+):(\d+)$`)
	reNewKey      = regexp.MustCompile(`fingerprint for the (\S+) key sent by the remote host is\s+(\S+)`)
)

// classifySSHError explains ssh's stderr after it exited with its own error status (255).
// fallback becomes Output when ssh printed nothing.
func classifySSHError(stderr, fallback string) *domain.SSHError {
	text := strings.ReplaceAll(stderr, "\r", "")
	e := &domain.SSHError{Reason: "SSH connection failed", Output: lastLines(text, outputLines)}
	if e.Output == "" {
		e.Output = fallback
	}
	for _, f := range sshFailures {
		for _, m := range f.match {
			if strings.Contains(text, m) {
				e.Reason, e.Hint = f.reason, f.hint
				if f.reason == "Host key changed" {
					e.HostKey = hostKeyChange(text)
				}
				return e
			}
		}
	}
	return e
}

// hostKeyChange reads the host and file to fix from ssh's warning; nil unless both are present,
// so lazyssh never guesses which known_hosts entry to remove.
func hostKeyChange(text string) *domain.HostKeyChange {
	host := reChangedHost.FindStringSubmatch(text)
	off := reOffending.FindStringSubmatch(text)
	if host == nil || off == nil {
		return nil
	}
	hk := &domain.HostKeyChange{Host: host[1], File: off[1]}
	hk.Line, _ = strconv.Atoi(off[2])
	if k := reNewKey.FindStringSubmatch(text); k != nil {
		hk.KeyType, hk.Fingerprint = k[1], strings.TrimSuffix(k[2], ".")
	}
	return hk
}

// lastLines returns the last n non-blank lines of s.
func lastLines(s string, n int) string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}

// storedPasswordError reports a stored password that could not be read before ssh even ran.
func storedPasswordError(err error) *domain.SSHError {
	return &domain.SSHError{
		Reason: "Stored password unavailable",
		Hint:   "Re-enter the password in Edit server, or restore ~/.lazyssh/master.key.",
		Output: err.Error(),
	}
}
