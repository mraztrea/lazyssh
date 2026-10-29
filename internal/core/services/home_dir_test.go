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
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

func TestHomeDirCommand(t *testing.T) {
	const sh = `exec "$SHELL" -l`
	cases := []struct {
		dir, remote, want string
	}{
		{"", "", ""},
		{"   ", "", ""},
		{"/var/www/app", "", `cd '/var/www/app'; ` + sh},
		{" /var/www/app ", "", `cd '/var/www/app'; ` + sh},
		{"~", "", `cd "$HOME"; ` + sh},
		{"~/a b", "", `cd "$HOME"/'a b'; ` + sh},
		{"/it's", "", `cd '/it'\''s'; ` + sh},
		{"/x; rm -rf /", "", `cd '/x; rm -rf /'; ` + sh},
		{"/x", "tmux attach || tmux new", `cd '/x'; tmux attach || tmux new`},
		{"/x", "none", `cd '/x'; ` + sh},
		{"", "tmux attach", ""},
	}
	for _, c := range cases {
		got := HomeDirCommand(domain.Server{HomeDir: c.dir, RemoteCommand: c.remote})
		if got != c.want {
			t.Errorf("HomeDirCommand(%q, %q) = %q, want %q", c.dir, c.remote, got, c.want)
		}
	}
}

func TestResolveRemoteDir(t *testing.T) {
	const home = "/home/u"
	cases := map[string]string{
		"":          home,
		"  ":        home,
		"~":         home,
		"~/a":       "/home/u/a",
		"a/b":       "/home/u/a/b",
		"/a/../b":   "/b",
		"/var/www/": "/var/www",
	}
	for in, want := range cases {
		if got := ResolveRemoteDir(in, home); got != want {
			t.Errorf("ResolveRemoteDir(%q) = %q, want %q", in, got, want)
		}
	}
}
