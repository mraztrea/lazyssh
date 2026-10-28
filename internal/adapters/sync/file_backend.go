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

// Package sync holds the sync backends that store the encrypted bundle (local file, GitHub gist).
package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Adembc/lazyssh/internal/core/ports"
)

type fileBackend struct {
	path string
}

// NewFileBackend stores the blob at path (a leading ~ is expanded to the home directory).
func NewFileBackend(path string) ports.SyncBackend {
	return &fileBackend{path: expandHome(path)}
}

func expandHome(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}

func (f *fileBackend) Upload(_ context.Context, blob []byte) (string, error) {
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create sync dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".lazyssh-sync-*") // created 0600
	if err != nil {
		return "", fmt.Errorf("create temp sync file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename
	if _, err := tmp.Write(blob); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write sync file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("write sync file: %w", err)
	}
	if err := os.Rename(tmp.Name(), f.path); err != nil {
		return "", fmt.Errorf("replace sync file: %w", err)
	}
	return f.path, nil
}

func (f *fileBackend) Download(_ context.Context) ([]byte, error) {
	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ports.ErrNoRemote, f.path)
	}
	if err != nil {
		return nil, fmt.Errorf("read sync file: %w", err)
	}
	return data, nil
}
