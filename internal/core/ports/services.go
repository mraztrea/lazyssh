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
	"time"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/pkg/sftp"
)

type ServerService interface {
	ListServers(query string) ([]domain.Server, error)
	UpdateServer(server domain.Server, newServer domain.Server) error
	AddServer(server domain.Server) error
	DeleteServer(server domain.Server) error
	SetPinned(alias string, pinned bool) error
	SSH(server domain.Server) error
	SSHWithArgs(server domain.Server, extraArgs []string) error
	StartForward(alias string, extraArgs []string) (int, error)
	StopForwarding(alias string) error
	IsForwarding(alias string) bool
	Ping(server domain.Server) (bool, time.Duration, error)
}

// SyncService pushes and pulls the encrypted config bundle. The password is never stored.
type SyncService interface {
	Settings() (domain.Settings, error)
	SaveSettings(domain.Settings) error
	Push(ctx context.Context, password string) error
	Pull(ctx context.Context, password string) error
}

// SFTPSession is an open SFTP client backed by an ssh subprocess; Close ends both.
type SFTPSession interface {
	Client() *sftp.Client
	Close() error
}

// SFTPService opens SFTP sessions for an SSH config alias.
type SFTPService interface {
	Connect(alias string) (SFTPSession, error)
}
