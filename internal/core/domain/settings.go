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

import "time"

// Settings is machine-local app configuration stored in ~/.lazyssh/settings.json.
type Settings struct {
	Editor string       `json:"editor,omitempty"` // command for the file manager's `e` key; empty means $EDITOR, then vi
	Sync   SyncSettings `json:"sync"`
}

// SyncSettings configures where the encrypted sync bundle is pushed to and pulled from.
type SyncSettings struct {
	Provider   string    `json:"provider,omitempty"` // "file" | "gist"
	FilePath   string    `json:"file_path,omitempty"`
	GistID     string    `json:"gist_id,omitempty"`
	GistToken  string    `json:"gist_token,omitempty"`
	LastPushAt time.Time `json:"last_push_at,omitzero"`
	LastPullAt time.Time `json:"last_pull_at,omitzero"`
}
