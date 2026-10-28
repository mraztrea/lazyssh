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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"go.uber.org/zap"
)

func newSnapshotRepo(t *testing.T) (repo *Repository, cfg, meta string) {
	t.Helper()
	cfg = filepath.Join(t.TempDir(), "config")
	meta = filepath.Join(t.TempDir(), "lazyssh", "metadata.json")
	return NewRepository(zap.NewNop().Sugar(), cfg, meta), cfg, meta
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestExport_ReadsConfigAndMetadata(t *testing.T) {
	repo, cfg, meta := newSnapshotRepo(t)
	cfgText := "Host a\n  HostName 1.1.1.1\n"
	metaText := `{"a":{"tags":["x"]}}`
	writeFile(t, cfg, cfgText)
	writeFile(t, meta, metaText)

	b, err := repo.Export()
	if err != nil {
		t.Fatal(err)
	}
	if b.Version != 1 || b.SSHConfig != cfgText || string(b.Metadata) != metaText || b.ExportedAt.IsZero() {
		t.Fatalf("unexpected bundle: %+v", b)
	}
}

func TestExport_MissingFilesGiveEmpty(t *testing.T) {
	repo, _, _ := newSnapshotRepo(t)
	b, err := repo.Export()
	if err != nil {
		t.Fatal(err)
	}
	if b.SSHConfig != "" || string(b.Metadata) != "{}" {
		t.Fatalf("unexpected bundle: %+v", b)
	}
}

func TestImport_WritesBothAndBacksUpConfig(t *testing.T) {
	repo, cfg, meta := newSnapshotRepo(t)
	writeFile(t, cfg, "Host old\n")

	err := repo.Import(domain.SyncBundle{Version: 1, SSHConfig: "Host new\n", Metadata: json.RawMessage(`{"new":{}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, cfg); got != "Host new\n" {
		t.Fatalf("config = %q", got)
	}
	if got := readFile(t, meta); got != `{"new":{}}` {
		t.Fatalf("metadata = %q", got)
	}
	for _, p := range []string{cfg, meta} {
		if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("%s: %v %v", p, fi, err)
		}
	}

	entries, err := os.ReadDir(filepath.Dir(cfg))
	if err != nil {
		t.Fatal(err)
	}
	var backup string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), BackupSuffix) {
			backup = filepath.Join(filepath.Dir(cfg), e.Name())
		}
	}
	if backup == "" {
		t.Fatal("no backup file created")
	}
	if got := readFile(t, backup); got != "Host old\n" {
		t.Fatalf("backup = %q", got)
	}
}

func TestImport_RejectsUnknownVersion(t *testing.T) {
	repo, cfg, _ := newSnapshotRepo(t)
	writeFile(t, cfg, "Host old\n")
	if err := repo.Import(domain.SyncBundle{Version: 2, SSHConfig: "Host new\n", Metadata: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("expected error for version 2")
	}
	if got := readFile(t, cfg); got != "Host old\n" {
		t.Fatalf("config changed: %q", got)
	}
}
