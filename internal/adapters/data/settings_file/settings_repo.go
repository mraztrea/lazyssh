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

package settings_file

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
)

type repository struct {
	path string
}

// NewRepository returns a settings repository backed by the JSON file at path.
func NewRepository(path string) ports.SettingsRepository {
	return &repository{path: path}
}

func (r *repository) Load() (domain.Settings, error) {
	var s domain.Settings
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("read settings '%s': %w", r.path, err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("parse settings '%s': %w", r.path, err)
	}
	return s, nil
}

func (r *repository) Save(s domain.Settings) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := os.WriteFile(r.path, data, 0o600); err != nil {
		return fmt.Errorf("write settings '%s': %w", r.path, err)
	}
	return nil
}
