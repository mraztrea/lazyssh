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
	var gotMeta map[string]ServerMetadata
	if err := json.Unmarshal([]byte(readFile(t, meta)), &gotMeta); err != nil {
		t.Fatal(err)
	}
	if _, ok := gotMeta["new"]; !ok || len(gotMeta) != 1 {
		t.Fatalf("metadata = %v", gotMeta)
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

func bundleMetadata(t *testing.T, b domain.SyncBundle) map[string]ServerMetadata {
	t.Helper()
	var m map[string]ServerMetadata
	if err := json.Unmarshal(b.Metadata, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func plainBundle(t *testing.T, pw, key string) domain.SyncBundle {
	t.Helper()
	meta, err := json.Marshal(map[string]ServerMetadata{"a": {Password: pw, PrivateKey: key}})
	if err != nil {
		t.Fatal(err)
	}
	return domain.SyncBundle{Version: 1, SSHConfig: "Host a\n  HostName h\n", Metadata: meta}
}

func TestExport_DecryptsSecrets(t *testing.T) {
	repo, _, _ := newSnapshotRepo(t)
	pw, key := genPassword(t), genPrivateKey(t)
	if err := repo.AddServer(domain.Server{Alias: "a", Host: "h", Password: pw, PrivateKey: key}); err != nil {
		t.Fatal(err)
	}
	b, err := repo.Export()
	if err != nil {
		t.Fatal(err)
	}
	if got := bundleMetadata(t, b)["a"]; got.Password != pw || got.PrivateKey != key {
		t.Fatal("bundle does not carry plaintext secrets")
	}
}

func TestExport_FailsOnUndecryptable(t *testing.T) {
	repo, _, meta := newSnapshotRepo(t)
	if err := repo.AddServer(domain.Server{Alias: "a", Host: "h", Password: genPassword(t)}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(masterKeyPath(meta)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Export(); err == nil || !strings.Contains(err.Error(), "master.key") {
		t.Fatalf("want error naming master.key, got %v", err)
	}
}

func TestImport_SealsWithLocalKey(t *testing.T) {
	repo, _, meta := newSnapshotRepo(t)
	pw, key := genPassword(t), genPrivateKey(t)
	if err := repo.Import(plainBundle(t, pw, key)); err != nil {
		t.Fatal(err)
	}
	raw := readFile(t, meta)
	assertNoSecretIn(t, raw, pw, key)
	if strings.Count(raw, "enc:v1:") != 2 {
		t.Fatalf("want 2 sealed values in metadata.json")
	}
	if s := onlyServer(t, repo); s.Password != pw || s.PrivateKey != key {
		t.Fatal("imported secrets do not round trip")
	}
}

func TestImport_CreatesMasterKeyIfMissing(t *testing.T) {
	repo, _, meta := newSnapshotRepo(t)
	assertNoFile(t, masterKeyPath(meta))
	if err := repo.Import(plainBundle(t, genPassword(t), "")); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(masterKeyPath(meta)); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("master.key: %v %v", fi, err)
	}
}

func TestImport_RejectsSealedValuesBeforeWrite(t *testing.T) {
	repo, cfg, meta := newSnapshotRepo(t)
	writeFile(t, cfg, "Host old\n")
	writeFile(t, meta, "{}")
	b := plainBundle(t, "enc:v1:AAAA", "")
	err := repo.Import(b)
	if err == nil || err.Error() != "sync bundle contains encrypted values from another machine" {
		t.Fatalf("got %v", err)
	}
	if readFile(t, cfg) != "Host old\n" || readFile(t, meta) != "{}" {
		t.Fatal("files changed after rejected import")
	}
	entries, err := os.ReadDir(filepath.Dir(cfg))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), BackupSuffix) {
			t.Fatalf("backup created: %s", e.Name())
		}
	}
	assertNoFile(t, masterKeyPath(meta))
}

func TestExportImport_DifferentMasterKeys(t *testing.T) {
	src, _, srcMeta := newSnapshotRepo(t)
	pw, key := genPassword(t), genPrivateKey(t)
	if err := src.AddServer(domain.Server{Alias: "a", Host: "h", Password: pw, PrivateKey: key}); err != nil {
		t.Fatal(err)
	}
	b, err := src.Export()
	if err != nil {
		t.Fatal(err)
	}

	dst, _, dstMeta := newSnapshotRepo(t)
	if err := dst.Import(b); err != nil {
		t.Fatal(err)
	}
	if s := onlyServer(t, dst); s.Password != pw || s.PrivateKey != key {
		t.Fatal("secrets lost across machines")
	}
	if readFile(t, masterKeyPath(srcMeta)) == readFile(t, masterKeyPath(dstMeta)) {
		t.Fatal("both machines share one master key")
	}
}
