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
	"fmt"
	"path/filepath"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/kevinburke/ssh_config"
	"go.uber.org/zap"
)

// Repository implements ServerRepository interface for SSH config file operations.
type Repository struct {
	configPath      string
	fileSystem      FileSystem
	metadataManager *metadataManager
	keys            keyStore
	logger          *zap.SugaredLogger
}

// NewRepository creates a new SSH config repository.
func NewRepository(logger *zap.SugaredLogger, configPath, metaDataPath string) *Repository {
	return &Repository{
		logger:          logger,
		configPath:      configPath,
		fileSystem:      DefaultFileSystem{},
		metadataManager: newMetadataManager(metaDataPath, logger),
		keys:            keyStore{dir: filepath.Join(filepath.Dir(metaDataPath), "keys")},
	}
}

// NewRepositoryWithFS creates a new SSH config repository with a custom filesystem.
func NewRepositoryWithFS(logger *zap.SugaredLogger, configPath string, metaDataPath string, fs FileSystem) *Repository {
	return &Repository{
		logger:          logger,
		configPath:      configPath,
		fileSystem:      fs,
		metadataManager: newMetadataManager(metaDataPath, logger),
		keys:            keyStore{dir: filepath.Join(filepath.Dir(metaDataPath), "keys")},
	}
}

// ListServers returns all servers matching the query pattern.
// Empty query returns all servers.
func (r *Repository) ListServers(query string) ([]domain.Server, error) {
	cfg, err := r.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	servers := r.toDomainServer(cfg)
	metadata, err := r.metadataManager.loadAll()
	if err != nil {
		return nil, fmt.Errorf("load metadata: %w", err)
	}
	servers = r.mergeMetadata(servers, metadata)
	for _, s := range servers {
		if s.PrivateKey == "" {
			continue
		}
		if err := r.keys.ensure(s.Alias, s.PrivateKey); err != nil {
			r.logger.Warnw("failed to materialize embedded private key", "alias", s.Alias, "error", err)
		}
	}
	if query == "" {
		return servers, nil
	}

	return r.filterServers(servers, query), nil
}

// AddServer adds a new server to the SSH config.
func (r *Repository) AddServer(server domain.Server) error {
	cfg, err := r.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if r.serverExists(cfg, server.Alias) {
		return fmt.Errorf("server with alias '%s' already exists", server.Alias)
	}

	if server.PrivateKey != "" {
		if err := r.keys.write(server.Alias, server.PrivateKey); err != nil {
			return fmt.Errorf("failed to write private key file: %w", err)
		}
		server.IdentityFiles = withKeyPath(server.IdentityFiles, r.keys.path(server.Alias))
	}

	host := r.createHostFromServer(server)
	cfg.Hosts = append(cfg.Hosts, host)

	if err := r.saveConfig(cfg); err != nil {
		r.logger.Warnf("Failed to save config while adding new server: %v", err)
		return fmt.Errorf("failed to save config: %w", err)
	}
	return r.metadataManager.updateServer(server, server.Alias)
}

// UpdateServer updates an existing server in the SSH config.
func (r *Repository) UpdateServer(server domain.Server, newServer domain.Server) error {
	cfg, err := r.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	host := r.findHostByAlias(cfg, server.Alias)
	if host == nil {
		return fmt.Errorf("server with alias '%s' not found", server.Alias)
	}

	if server.Alias != newServer.Alias {
		if r.serverExists(cfg, newServer.Alias) {
			return fmt.Errorf("server with alias '%s' already exists", newServer.Alias)
		}

		newPatterns := make([]*ssh_config.Pattern, 0, len(host.Patterns))
		for _, pattern := range host.Patterns {
			if pattern.Str == server.Alias {
				newPatterns = append(newPatterns, &ssh_config.Pattern{Str: newServer.Alias})
			} else {
				newPatterns = append(newPatterns, pattern)
			}
		}

		host.Patterns = newPatterns

	}

	// A locked server saved without a new secret keeps its materialized key file too.
	if server.SecretError == "" || newServer.PrivateKey != "" || newServer.Password != "" {
		if err := r.syncKeyFile(server.Alias, &newServer); err != nil {
			return err
		}
	}

	r.updateHostNodes(host, newServer)

	if err := r.saveConfig(cfg); err != nil {
		r.logger.Warnf("Failed to save config while updating server: %v", err)
		return fmt.Errorf("failed to save config: %w", err)
	}
	// Update metadata; pass old alias to allow inline migration
	return r.metadataManager.updateServer(newServer, server.Alias)
}

// DeleteServer removes a server from the SSH config.
func (r *Repository) DeleteServer(server domain.Server) error {
	cfg, err := r.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	initialCount := len(cfg.Hosts)
	cfg.Hosts = r.removeHostByAlias(cfg.Hosts, server.Alias)

	if len(cfg.Hosts) == initialCount {
		return fmt.Errorf("server with alias '%s' not found", server.Alias)
	}

	if err := r.saveConfig(cfg); err != nil {
		r.logger.Warnf("Failed to save config while deleting server: %v", err)
		return fmt.Errorf("failed to save config: %w", err)
	}
	if err := r.metadataManager.deleteServer(server.Alias); err != nil {
		return err
	}
	return r.keys.remove(server.Alias)
}

// syncKeyFile brings the key file and IdentityFiles of newServer in line with its PrivateKey,
// handling an alias change from oldAlias.
func (r *Repository) syncKeyFile(oldAlias string, newServer *domain.Server) error {
	oldPath := r.keys.path(oldAlias)
	if newServer.PrivateKey == "" {
		if err := r.keys.remove(oldAlias); err != nil {
			return fmt.Errorf("failed to remove private key file: %w", err)
		}
		newServer.IdentityFiles = withoutKeyPath(newServer.IdentityFiles, oldPath)
		return nil
	}
	if oldAlias != newServer.Alias {
		if err := r.keys.remove(oldAlias); err != nil {
			return fmt.Errorf("failed to remove private key file: %w", err)
		}
	}
	if err := r.keys.write(newServer.Alias, newServer.PrivateKey); err != nil {
		return fmt.Errorf("failed to write private key file: %w", err)
	}
	newServer.IdentityFiles = withKeyPath(withoutKeyPath(newServer.IdentityFiles, oldPath), r.keys.path(newServer.Alias))
	return nil
}

// SetPinned sets or unsets the pinned status of a server.
func (r *Repository) SetPinned(alias string, pinned bool) error {
	return r.metadataManager.setPinned(alias, pinned)
}

// RecordSSH increments the SSH access count and updates the last seen timestamp for a server.
func (r *Repository) RecordSSH(alias string) error {
	return r.metadataManager.recordSSH(alias)
}

// GetPassword returns the stored password for alias ("" if none). It only reads metadata.json.
func (r *Repository) GetPassword(alias string) (string, error) {
	metadata, err := r.metadataManager.loadAll()
	if err != nil {
		return "", err
	}
	meta := metadata[alias]
	if meta.SecretError != "" {
		return "", fmt.Errorf("%s", meta.SecretError)
	}
	return meta.Password, nil
}
