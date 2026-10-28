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
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"go.uber.org/zap"
)

const testKey = "-----BEGIN X-----\nabc\n-----END X-----\n"

// newKeyTestRepo returns a repository on temp files and the expected keys directory.
func newKeyTestRepo(t *testing.T) (ports.ServerRepository, string) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config")
	metaDir := t.TempDir()
	repo := NewRepository(zap.NewNop().Sugar(), cfg, filepath.Join(metaDir, "metadata.json"))
	return repo, filepath.Join(metaDir, "keys")
}

func addWebWithKey(t *testing.T, repo ports.ServerRepository, keys string) {
	t.Helper()
	if err := repo.AddServer(domain.Server{Alias: "web", Host: "h", PrivateKey: testKey}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(keys, "web")); err != nil {
		t.Fatalf("precondition: key file not written: %v", err)
	}
}

func onlyServer(t *testing.T, repo ports.ServerRepository) domain.Server {
	t.Helper()
	servers, err := repo.ListServers("")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("want 1 server, got %d", len(servers))
	}
	return servers[0]
}

func assertNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s should not exist (err=%v)", path, err)
	}
}

func TestAddServer_WithPrivateKey_WritesFileAndIdentityFile(t *testing.T) {
	repo, keys := newKeyTestRepo(t)
	addWebWithKey(t, repo, keys)

	keyPath := filepath.Join(keys, "web")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key file mode = %o, want 600", info.Mode().Perm())
	}
	if dirInfo, err := os.Stat(keys); err != nil || dirInfo.Mode().Perm() != 0o700 {
		t.Errorf("keys dir mode = %v (err=%v), want 700", dirInfo.Mode().Perm(), err)
	}
	if got, _ := os.ReadFile(keyPath); string(got) != testKey {
		t.Errorf("key file content = %q", got)
	}

	s := onlyServer(t, repo)
	if len(s.IdentityFiles) == 0 || s.IdentityFiles[0] != keyPath {
		t.Errorf("IdentityFiles = %v, want first %s", s.IdentityFiles, keyPath)
	}
	if s.PrivateKey != testKey {
		t.Errorf("PrivateKey = %q", s.PrivateKey)
	}
}

func TestUpdateServer_ClearPrivateKey_RemovesFileAndIdentityFile(t *testing.T) {
	repo, keys := newKeyTestRepo(t)
	addWebWithKey(t, repo, keys)
	old := onlyServer(t, repo)

	updated := old
	updated.PrivateKey = ""
	if err := repo.UpdateServer(old, updated); err != nil {
		t.Fatal(err)
	}

	assertNoFile(t, filepath.Join(keys, "web"))
	s := onlyServer(t, repo)
	if len(s.IdentityFiles) != 0 {
		t.Errorf("IdentityFiles = %v, want empty", s.IdentityFiles)
	}
	if s.PrivateKey != "" {
		t.Errorf("PrivateKey not cleared")
	}
}

func TestUpdateServer_RenameAlias_MovesKeyFile(t *testing.T) {
	repo, keys := newKeyTestRepo(t)
	addWebWithKey(t, repo, keys)
	old := onlyServer(t, repo)

	renamed := old
	renamed.Alias = "web2"
	if err := repo.UpdateServer(old, renamed); err != nil {
		t.Fatal(err)
	}

	assertNoFile(t, filepath.Join(keys, "web"))
	if got, err := os.ReadFile(filepath.Join(keys, "web2")); err != nil || string(got) != testKey {
		t.Fatalf("keys/web2 = %q, err=%v", got, err)
	}
	s := onlyServer(t, repo)
	if want := []string{filepath.Join(keys, "web2")}; !reflect.DeepEqual(s.IdentityFiles, want) {
		t.Errorf("IdentityFiles = %v, want %v", s.IdentityFiles, want)
	}
	if s.PrivateKey != testKey {
		t.Errorf("PrivateKey lost on rename")
	}
}

func TestDeleteServer_RemovesKeyFile(t *testing.T) {
	repo, keys := newKeyTestRepo(t)
	addWebWithKey(t, repo, keys)

	if err := repo.DeleteServer(domain.Server{Alias: "web"}); err != nil {
		t.Fatal(err)
	}
	assertNoFile(t, filepath.Join(keys, "web"))
}

func TestListServers_RecreatesMissingKeyFile(t *testing.T) {
	repo, keys := newKeyTestRepo(t)
	addWebWithKey(t, repo, keys)
	keyPath := filepath.Join(keys, "web")
	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}

	onlyServer(t, repo)

	if got, err := os.ReadFile(keyPath); err != nil || string(got) != testKey {
		t.Fatalf("key not recreated: %q, err=%v", got, err)
	}
}
