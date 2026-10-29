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

import "os"

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
