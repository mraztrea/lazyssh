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
	"path"
	"strings"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

// shellQuote single-quotes s so a POSIX shell reads it verbatim.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// homeDirExpr renders dir for the remote shell; a leading ~ becomes "$HOME" so it still expands.
func homeDirExpr(dir string) string {
	switch {
	case dir == "~":
		return `"$HOME"`
	case strings.HasPrefix(dir, "~/"):
		return `"$HOME"/` + shellQuote(dir[2:])
	default:
		return shellQuote(dir)
	}
}

// HomeDirCommand is the remote command that starts an ssh session in s.HomeDir: s.RemoteCommand
// when set, else the user's login shell. It is "" when no home dir is set.
func HomeDirCommand(s domain.Server) string {
	dir := strings.TrimSpace(s.HomeDir)
	if dir == "" {
		return ""
	}
	rest := `exec "$SHELL" -l`
	if s.RemoteCommand != "" && s.RemoteCommand != "none" {
		rest = s.RemoteCommand
	}
	// ";" rather than "&&": a missing dir still lands the user in a shell, with cd's error shown.
	return "cd " + homeDirExpr(dir) + "; " + rest
}

// ResolveRemoteDir turns a home dir setting into an absolute SFTP path; ~ and relative paths are
// taken from home, the session's starting directory.
func ResolveRemoteDir(dir, home string) string {
	dir = strings.TrimSpace(dir)
	switch {
	case dir == "" || dir == "~":
		return home
	case strings.HasPrefix(dir, "~/"):
		return path.Join(home, dir[2:])
	case !strings.HasPrefix(dir, "/"):
		return path.Join(home, dir)
	default:
		return path.Clean(dir)
	}
}
