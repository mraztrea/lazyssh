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
	"crypto/rand"
	"encoding/base64"
	"errors"
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
