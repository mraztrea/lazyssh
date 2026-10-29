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

package ports

import (
	"context"
	"errors"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

type ServerRepository interface {
	ListServers(query string) ([]domain.Server, error)
	UpdateServer(server domain.Server, newServer domain.Server) error
	AddServer(server domain.Server) error
	DeleteServer(server domain.Server) error
	SetPinned(alias string, pinned bool) error
	RecordSSH(alias string) error
	GetPassword(alias string) (string, error)
}

// SettingsRepository loads and saves app settings. A missing store loads as the zero value.
type SettingsRepository interface {
	Load() (domain.Settings, error)
	Save(domain.Settings) error
}

// SnapshotRepository exports and imports the whole local config as a sync bundle.
type SnapshotRepository interface {
	Export() (domain.SyncBundle, error)
	Import(domain.SyncBundle) error
}

// ErrNoRemote is returned by SyncBackend.Download when nothing has been pushed yet.
var ErrNoRemote = errors.New("no remote sync data found")

// SyncBackend stores one encrypted sync blob somewhere (a file, a gist).
// Upload returns the identifier the backend wants persisted (gist id, file path).
type SyncBackend interface {
	Upload(ctx context.Context, blob []byte) (remoteID string, err error)
	Download(ctx context.Context) ([]byte, error)
}
