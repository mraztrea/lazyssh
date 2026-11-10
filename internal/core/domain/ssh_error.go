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

package domain

// SSHError is a failed ssh/sftp connection, explained for the user.
type SSHError struct {
	Reason  string         // one-line cause, e.g. "Connection refused"
	Hint    string         // what the user can do about it
	Output  string         // last lines ssh printed, verbatim
	HostKey *HostKeyChange // set only when the server's host key changed and ssh named host and file
}

func (e *SSHError) Error() string { return e.Reason }

// HostKeyChange is what ssh reported about a host key that no longer matches known_hosts.
type HostKeyChange struct {
	Host        string // known_hosts name, e.g. "1.2.3.4" or "[1.2.3.4]:2222"
	File        string // known_hosts file holding the old key
	Line        int
	KeyType     string // type of the new key, e.g. "ED25519"
	Fingerprint string // fingerprint of the new key
}
