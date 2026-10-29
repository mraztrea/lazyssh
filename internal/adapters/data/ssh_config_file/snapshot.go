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

package ssh_config_file

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/Adembc/lazyssh/internal/secretbox"
)

var (
	_ ports.ServerRepository   = (*Repository)(nil)
	_ ports.SnapshotRepository = (*Repository)(nil)
)

// Export returns the raw SSH config and the decrypted metadata as a sync bundle (the bundle itself
// is encrypted by the sync service).
func (r *Repository) Export() (domain.SyncBundle, error) {
	b := domain.SyncBundle{Version: domain.SyncBundleVersion, ExportedAt: time.Now().UTC()}
	b.Hostname, _ = os.Hostname()

	f, err := r.fileSystem.Open(r.configPath)
	switch {
	case err == nil:
		data, readErr := io.ReadAll(f)
		_ = f.Close()
		if readErr != nil {
			return b, fmt.Errorf("read ssh config: %w", readErr)
		}
		b.SSHConfig = string(data)
	case !r.fileSystem.IsNotExist(err):
		return b, fmt.Errorf("open ssh config: %w", err)
	}

	meta, err := r.metadataManager.loadAll()
	if err != nil {
		return b, fmt.Errorf("load metadata: %w", err)
	}
	if b.Metadata, err = json.Marshal(meta); err != nil {
		return b, fmt.Errorf("marshal metadata: %w", err)
	}
	return b, nil
}

// Import replaces the SSH config (after the usual backups) and the metadata file with the bundle,
// sealing its plaintext secrets with this machine's master key.
// ponytail: two separate atomic renames, not one transaction; a crash between them leaves new
// config with old metadata. Pull again to recover.
func (r *Repository) Import(b domain.SyncBundle) error {
	if b.Version != domain.SyncBundleVersion {
		return fmt.Errorf("unsupported sync bundle version %d", b.Version)
	}
	meta := make(map[string]ServerMetadata)
	if len(bytes.TrimSpace(b.Metadata)) > 0 {
		if err := json.Unmarshal(b.Metadata, &meta); err != nil {
			return fmt.Errorf("parse sync bundle metadata: %w", err)
		}
	}
	for _, m := range meta {
		if secretbox.IsSealed(m.PrivateKey) || secretbox.IsSealed(m.Password) {
			return errors.New("sync bundle contains encrypted values from another machine")
		}
	}
	if err := r.saveRawConfig(b.SSHConfig); err != nil {
		return err
	}

	if err := r.metadataManager.ensureDirectory(); err != nil {
		return err
	}
	data, err := r.metadataManager.marshalSealed(meta)
	if err != nil {
		return fmt.Errorf("seal metadata: %w", err)
	}
	path := r.metadataManager.filePath
	tmp := path + TempSuffix
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace metadata: %w", err)
	}
	r.logger.Infow("imported sync bundle", "config", r.configPath, "metadata", path)
	return nil
}
