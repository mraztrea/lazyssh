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
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// executable is the askpass helper path; a variable so tests can pin it.
var executable = os.Executable

// passwordAuth returns the extra ssh args and the full environment that make ssh ask lazyssh
// (as SSH_ASKPASS) for alias's stored password. Both are nil when alias has no password.
// The password itself never appears in args or env.
func passwordAuth(lookup func(alias string) (string, error), alias string) (args, env []string, err error) {
	pw, err := lookup(alias)
	if err != nil {
		return nil, nil, err
	}
	if pw == "" {
		return nil, nil, nil
	}
	exe, err := executable()
	if err != nil {
		return nil, nil, err
	}
	args = []string{"-o", "PreferredAuthentications=password,keyboard-interactive", "-o", "NumberOfPasswordPrompts=1"}
	env = append(os.Environ(), "SSH_ASKPASS="+exe, "SSH_ASKPASS_REQUIRE=force", "LAZYSSH_ASKPASS_ALIAS="+alias)
	return args, env, nil
}

// Askpass is the SSH_ASKPASS helper (contracts/askpass.md). It prints alias's stored password only
// for a password prompt naming alias's own user@host; every other prompt is relayed to the tty.
// Returns the process exit code. stderr messages never carry a secret.
func Askpass(
	lookup func(alias string) (string, error),
	userHost func(alias string) (user, host string, err error),
	alias, prompt string,
	out io.Writer,
	tty func() (io.ReadWriteCloser, error),
) int {
	p := strings.ToLower(strings.TrimRight(prompt, " \t"))
	if strings.HasSuffix(p, "password:") {
		// OpenSSH prompts are exactly "user@host's password: " (password auth) or
		// "(user@host) <prompt>" (keyboard-interactive); anchor on them so a jump host whose
		// user@host merely contains ours never gets this password.
		user, host, err := userHost(alias)
		uh := strings.ToLower(user + "@" + host)
		if err == nil && user != "" && host != "" && (strings.HasPrefix(p, uh+"'s password:") || strings.HasPrefix(p, "("+uh+") ")) {
			pw, err := lookup(alias)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "lazyssh askpass: %s: %v\n", alias, err)
				return 1
			}
			if pw == "" {
				_, _ = fmt.Fprintf(os.Stderr, "lazyssh askpass: no stored password for %s\n", alias)
				return 1
			}
			_, _ = fmt.Fprintln(out, pw)
			return 0
		}
	}

	// shortcut: relayed input echoes (no x/term); fine for host-key yes/no and OTP codes, upgrade if passphrases ever reach the helper.
	t, err := tty()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lazyssh askpass: %v\n", err)
		return 1
	}
	defer func() { _ = t.Close() }()
	if _, err := io.WriteString(t, prompt); err != nil {
		return 1
	}
	line, err := bufio.NewReader(t).ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return 1
	}
	_, _ = fmt.Fprintln(out, strings.TrimRight(line, "\r\n"))
	return 0
}
