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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// keyStore materializes embedded private keys as files so OpenSSH can use them via IdentityFile.
// Metadata is the source of truth; these files are a cache rebuilt by ListServers.
type keyStore struct{ dir string }

func (k keyStore) path(alias string) string {
	return filepath.Join(k.dir, alias)
}

func (k keyStore) write(alias, pem string) error {
	if err := os.MkdirAll(k.dir, 0o700); err != nil {
		return err
	}
	p := k.path(alias)
	if err := os.WriteFile(p, []byte(pem), 0o600); err != nil {
		return err
	}
	// WriteFile keeps the mode of an existing file; force 0600 regardless.
	return os.Chmod(p, 0o600)
}

func (k keyStore) remove(alias string) error {
	if err := os.Remove(k.path(alias)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// ensure writes the key only when the file is missing or its content differs.
func (k keyStore) ensure(alias, pem string) error {
	if got, err := os.ReadFile(k.path(alias)); err == nil && string(got) == pem {
		return nil
	}
	return k.write(alias, pem)
}

// withKeyPath prepends path to files unless already present.
func withKeyPath(files []string, path string) []string {
	if slices.Contains(files, path) {
		return files
	}
	return append([]string{path}, files...)
}

// withoutKeyPath returns files with every occurrence of path removed.
func withoutKeyPath(files []string, path string) []string {
	var out []string
	for _, f := range files {
		if f != path {
			out = append(out, f)
		}
	}
	return out
}
