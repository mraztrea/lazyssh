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
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/ports"
	"go.uber.org/zap"
)

var authOpts = []string{"-o", "PreferredAuthentications=password,keyboard-interactive", "-o", "NumberOfPasswordPrompts=1"}

func randomPassword(t *testing.T) string {
	t.Helper()
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func fakeExecutable(t *testing.T) {
	t.Helper()
	orig := executable
	executable = func() (string, error) { return "/opt/lazyssh", nil }
	t.Cleanup(func() { executable = orig })
}

func lookupOf(pw string, err error) func(string) (string, error) {
	return func(string) (string, error) { return pw, err }
}

// pwRepo is a ServerRepository that only answers GetPassword.
type pwRepo struct {
	ports.ServerRepository
	pw  string
	err error
}

func (r pwRepo) GetPassword(string) (string, error) { return r.pw, r.err }

func TestPasswordAuth_NoPasswordUnchanged(t *testing.T) {
	fakeExecutable(t)
	args, env, err := passwordAuth(lookupOf("", nil), "web")
	if err != nil || args != nil || env != nil {
		t.Fatalf("args=%v env=%v err=%v", args, env != nil, err)
	}
}

func TestPasswordAuth_SetsAskpassEnvAndOptions(t *testing.T) {
	fakeExecutable(t)
	args, env, err := passwordAuth(lookupOf(randomPassword(t), nil), "web")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, authOpts) {
		t.Fatalf("args = %v", args)
	}
	for _, want := range []string{"SSH_ASKPASS=/opt/lazyssh", "SSH_ASKPASS_REQUIRE=force", "LAZYSSH_ASKPASS_ALIAS=web"} {
		if !slices.Contains(env, want) {
			t.Fatalf("env missing %s", want)
		}
	}
}

func TestPasswordAuth_SecretNotInArgsOrEnv(t *testing.T) {
	fakeExecutable(t)
	pw := randomPassword(t)
	args, env, err := passwordAuth(lookupOf(pw, nil), "web")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range append(args, env...) {
		if strings.Contains(v, pw) {
			t.Fatal("password leaked into args or env")
		}
	}

	s := &serverService{logger: zap.NewNop().Sugar(), serverRepository: pwRepo{pw: pw}}
	cmd, err := s.sshCmd("web", []string{"-L", "1:h:2"}, []string{"-N"})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range append(cmd.Args, cmd.Env...) {
		if strings.Contains(v, pw) {
			t.Fatal("password leaked into ssh command")
		}
	}
}

func TestPasswordAuth_LookupErrorAborts(t *testing.T) {
	fakeExecutable(t)
	boom := errors.New("master key /x/master.key: corrupt")
	args, env, err := passwordAuth(lookupOf("", boom), "web")
	if !errors.Is(err, boom) || args != nil || env != nil {
		t.Fatalf("args=%v env=%v err=%v", args, env != nil, err)
	}

	s := &serverService{logger: zap.NewNop().Sugar(), serverRepository: pwRepo{err: boom}}
	if cmd, err := s.sshCmd("web", nil, nil); !errors.Is(err, boom) || cmd != nil {
		t.Fatalf("sshCmd did not abort: %v", err)
	}
}

func TestSSHCmd_OptionsBeforeDestination(t *testing.T) {
	fakeExecutable(t)
	extra := []string{"-L", "8080:localhost:80"}
	cases := []struct {
		name      string
		pre, post []string
		want      []string
	}{
		{"connect", nil, nil, []string{"ssh"}},
		{"with args", extra, nil, append([]string{"ssh"}, extra...)},
		{"forward", extra, []string{"-N"}, append([]string{"ssh"}, extra...)},
	}
	for _, c := range cases {
		pw := &serverService{logger: zap.NewNop().Sugar(), serverRepository: pwRepo{pw: randomPassword(t)}}
		cmd, err := pw.sshCmd("web", c.pre, c.post)
		if err != nil {
			t.Fatal(err)
		}
		want := slices.Concat(c.want, authOpts, c.post, []string{"web"})
		if !reflect.DeepEqual(cmd.Args, want) {
			t.Errorf("%s with password: args = %v, want %v", c.name, cmd.Args, want)
		}
		if !slices.Contains(cmd.Env, "LAZYSSH_ASKPASS_ALIAS=web") {
			t.Errorf("%s with password: askpass env missing", c.name)
		}

		plain := &serverService{logger: zap.NewNop().Sugar(), serverRepository: pwRepo{}}
		cmd, err = plain.sshCmd("web", c.pre, c.post)
		if err != nil {
			t.Fatal(err)
		}
		want = slices.Concat(c.want, c.post, []string{"web"})
		if !reflect.DeepEqual(cmd.Args, want) || cmd.Env != nil {
			t.Errorf("%s without password: args = %v env=%v, want %v", c.name, cmd.Args, cmd.Env != nil, want)
		}
	}
}

// memTTY is an in-memory /dev/tty: reads come from in, writes are recorded.
type memTTY struct {
	in      *strings.Reader
	written bytes.Buffer
	closed  bool
}

func (m *memTTY) Read(p []byte) (int, error)  { return m.in.Read(p) }
func (m *memTTY) Write(p []byte) (int, error) { return m.written.Write(p) }
func (m *memTTY) Close() error                { m.closed = true; return nil }

func ttyWith(input string) (*memTTY, func() (io.ReadWriteCloser, error)) {
	m := &memTTY{in: strings.NewReader(input)}
	return m, func() (io.ReadWriteCloser, error) { return m, nil }
}

func noTTY() (io.ReadWriteCloser, error) { return nil, errors.New("open /dev/tty: no such device") }

func aliceHost(string) (string, string, error) { return "alice", "10.0.0.5", nil }

func runAskpass(t *testing.T, lookup func(string) (string, error), userHost func(string) (string, string, error), prompt string, tty func() (io.ReadWriteCloser, error)) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code := Askpass(lookup, userHost, "web", prompt, &out, tty)
	return code, out.String()
}

func TestAskpass_PrintsPasswordForTargetPrompt(t *testing.T) {
	pw := randomPassword(t)
	code, out := runAskpass(t, lookupOf(pw, nil), aliceHost, "alice@10.0.0.5's password: ", noTTY)
	if code != 0 || out != pw+"\n" {
		t.Fatalf("code=%d, password not printed", code)
	}
}

func TestAskpass_KeyboardInteractivePromptAnswered(t *testing.T) {
	pw := randomPassword(t)
	code, out := runAskpass(t, lookupOf(pw, nil), aliceHost, "(alice@10.0.0.5) Password: ", noTTY)
	if code != 0 || out != pw+"\n" {
		t.Fatalf("code=%d, password not printed", code)
	}
}

// A jump host whose user@host merely contains ours (10.0.0.50 vs 10.0.0.5) must not get our password.
func TestAskpass_SuperstringHostPromptNotAnswered(t *testing.T) {
	pw := randomPassword(t)
	for _, prompt := range []string{"alice@10.0.0.50's password: ", "(alice@10.0.0.5x) Password: ", "xalice@10.0.0.5's password: "} {
		tty, open := ttyWith("typed\n")
		code, out := runAskpass(t, lookupOf(pw, nil), aliceHost, prompt, open)
		if code != 0 || out != "typed\n" || tty.written.String() != prompt {
			t.Fatalf("%q: code=%d out=%q tty=%q", prompt, code, out, tty.written.String())
		}
	}
}

func TestAskpass_OtherHostPromptNotAnswered(t *testing.T) {
	pw := randomPassword(t)
	prompt := "bob@jump's password: "
	tty, open := ttyWith("typed\n")
	code, out := runAskpass(t, lookupOf(pw, nil), aliceHost, prompt, open)
	if code != 0 || out != "typed\n" {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if tty.written.String() != prompt || !tty.closed {
		t.Fatalf("prompt not relayed to tty: %q", tty.written.String())
	}
	if strings.Contains(out, pw) || strings.Contains(tty.written.String(), pw) {
		t.Fatal("password sent to another host")
	}
}

func TestAskpass_HostKeyPromptRelayed(t *testing.T) {
	prompt := "Are you sure you want to continue connecting (yes/no/[fingerprint])? "
	tty, open := ttyWith("yes\r\n")
	code, out := runAskpass(t, lookupOf(randomPassword(t), nil), aliceHost, prompt, open)
	if code != 0 || out != "yes\n" || tty.written.String() != prompt {
		t.Fatalf("code=%d out=%q tty=%q", code, out, tty.written.String())
	}
}

func TestAskpass_NoTTYExitsNonZero(t *testing.T) {
	code, out := runAskpass(t, lookupOf(randomPassword(t), nil), aliceHost, "Enter OTP: ", noTTY)
	if code != 1 || out != "" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestAskpass_UnknownAliasExitsNonZero(t *testing.T) {
	known := map[string]string{"other": "x"}
	lookup := func(a string) (string, error) { return known[a], nil }
	code, out := runAskpass(t, lookup, aliceHost, "alice@10.0.0.5's password: ", noTTY)
	if code != 1 || out != "" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestAskpass_NoPasswordExitsNonZero(t *testing.T) {
	code, out := runAskpass(t, lookupOf("", nil), aliceHost, "alice@10.0.0.5's password: ", noTTY)
	if code != 1 || out != "" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestAskpass_LookupErrorExitsNonZeroWithoutSecret(t *testing.T) {
	pw := randomPassword(t)
	code, out := runAskpass(t, lookupOf(pw, errors.New("master key /h/.lazyssh/master.key is corrupt")), aliceHost, "alice@10.0.0.5's password: ", noTTY)
	if code != 1 || out != "" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestAskpass_UserHostErrorRelays(t *testing.T) {
	pw := randomPassword(t)
	failing := func(string) (string, string, error) { return "", "", errors.New("ssh -G failed") }
	tty, open := ttyWith("typed\n")
	code, out := runAskpass(t, lookupOf(pw, nil), failing, "alice@10.0.0.5's password: ", open)
	if code != 0 || out != "typed\n" || tty.written.Len() == 0 {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if strings.Contains(out, pw) {
		t.Fatal("password printed without a resolved destination")
	}
}
