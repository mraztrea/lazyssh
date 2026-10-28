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

import (
	"encoding/json"
	"time"
)

// SyncBundleVersion is the only bundle format version this build reads and writes.
const SyncBundleVersion = 1

// SyncBundle is the plaintext payload of a sync push: the SSH config plus lazyssh metadata
// (which carries embedded private keys). It is only ever stored or sent encrypted.
type SyncBundle struct {
	Version    int             `json:"version"`
	ExportedAt time.Time       `json:"exported_at"`
	Hostname   string          `json:"hostname"`
	SSHConfig  string          `json:"ssh_config"`
	Metadata   json.RawMessage `json:"metadata"`
}
