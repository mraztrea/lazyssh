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
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"go.uber.org/zap"
)

// ErrSyncNotConfigured means no (or an unknown) sync provider is selected.
var ErrSyncNotConfigured = errors.New("sync is not configured: choose provider file or gist")

type syncService struct {
	logger     *zap.SugaredLogger
	settings   ports.SettingsRepository
	snapshot   ports.SnapshotRepository
	backendFor func(domain.SyncSettings) (ports.SyncBackend, error)
}

// NewSyncService wires settings, the local snapshot and a backend factory into push/pull.
func NewSyncService(
	logger *zap.SugaredLogger,
	settings ports.SettingsRepository,
	snapshot ports.SnapshotRepository,
	backendFor func(domain.SyncSettings) (ports.SyncBackend, error),
) ports.SyncService {
	return &syncService{logger: logger, settings: settings, snapshot: snapshot, backendFor: backendFor}
}

func (s *syncService) Settings() (domain.Settings, error) { return s.settings.Load() }

func (s *syncService) SaveSettings(st domain.Settings) error { return s.settings.Save(st) }

// prepare validates the password and resolves settings and backend before anything is read or sent.
func (s *syncService) prepare(password string) (domain.Settings, ports.SyncBackend, error) {
	if password == "" {
		return domain.Settings{}, nil, ErrPasswordRequired
	}
	st, err := s.settings.Load()
	if err != nil {
		return st, nil, fmt.Errorf("load settings: %w", err)
	}
	if st.Sync.Provider == "" {
		return st, nil, ErrSyncNotConfigured
	}
	backend, err := s.backendFor(st.Sync)
	if err != nil {
		return st, nil, err
	}
	return st, backend, nil
}

// Push: Export → seal → Upload → record LastPushAt (and the gist id) → save settings.
func (s *syncService) Push(ctx context.Context, password string) error {
	st, backend, err := s.prepare(password)
	if err != nil {
		return err
	}
	bundle, err := s.snapshot.Export()
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	blob, err := sealBundle(bundle, password)
	if err != nil {
		return err
	}
	id, err := backend.Upload(ctx, blob)
	if err != nil {
		s.logger.Warnw("sync push failed", "provider", st.Sync.Provider, "error", err)
		return err
	}
	if st.Sync.Provider == "gist" {
		st.Sync.GistID = id
	}
	st.Sync.LastPushAt = time.Now().UTC()
	if err := s.settings.Save(st); err != nil {
		return fmt.Errorf("pushed, but saving settings failed: %w", err)
	}
	s.logger.Infow("sync push complete", "provider", st.Sync.Provider, "remote", id)
	return nil
}

// Pull: Download → open → Import → record LastPullAt → save settings.
// Decryption happens before Import, so a wrong password never touches local files.
func (s *syncService) Pull(ctx context.Context, password string) error {
	st, backend, err := s.prepare(password)
	if err != nil {
		return err
	}
	blob, err := backend.Download(ctx)
	if err != nil {
		s.logger.Warnw("sync pull failed", "provider", st.Sync.Provider, "error", err)
		return err
	}
	bundle, err := openBundle(blob, password)
	if err != nil {
		return err
	}
	if err := s.snapshot.Import(bundle); err != nil {
		return fmt.Errorf("import: %w", err)
	}
	st.Sync.LastPullAt = time.Now().UTC()
	if err := s.settings.Save(st); err != nil {
		return fmt.Errorf("pulled, but saving settings failed: %w", err)
	}
	s.logger.Infow("sync pull complete", "provider", st.Sync.Provider)
	return nil
}
