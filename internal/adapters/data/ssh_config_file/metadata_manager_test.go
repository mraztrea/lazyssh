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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// genPassword returns a random password generated at test time.
func genPassword(t *testing.T) string {
	t.Helper()
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// genPrivateKey returns a freshly generated OpenSSH private key in PEM form.
func genPrivateKey(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(block))
}

// newSecretRepo returns a repository whose metadata lives in <tmp>/.lazyssh.
func newSecretRepo(t *testing.T) (repo *Repository, cfg, meta string) {
	t.Helper()
	home := t.TempDir()
	cfg = filepath.Join(home, "config")
	meta = filepath.Join(home, ".lazyssh", "metadata.json")
	return NewRepository(zap.NewNop().Sugar(), cfg, meta), cfg, meta
}

func masterKeyPath(meta string) string { return filepath.Join(filepath.Dir(meta), "master.key") }

func rawMetadata(t *testing.T, meta string) map[string]map[string]any {
	t.Helper()
	var m map[string]map[string]any
	if err := json.Unmarshal([]byte(readFile(t, meta)), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func assertNoSecretIn(t *testing.T, s, password, key string) {
	t.Helper()
	if password != "" && strings.Contains(s, password) {
		t.Fatal("output contains the password")
	}
	for _, line := range strings.Split(key, "\n") {
		if line != "" && strings.Contains(s, line) {
			t.Fatal("output contains a private key line")
		}
	}
}

func TestMetadata_SecretsEncryptedOnDisk(t *testing.T) {
	repo, _, meta := newSecretRepo(t)
	pw, key := genPassword(t), genPrivateKey(t)
	if err := repo.AddServer(domain.Server{Alias: "a", Host: "h", PrivateKey: key, Password: pw}); err != nil {
		t.Fatal(err)
	}

	assertNoSecretIn(t, readFile(t, meta), pw, key)
	entry := rawMetadata(t, meta)["a"]
	for _, field := range []string{"private_key", "password"} {
		v, _ := entry[field].(string)
		if !strings.HasPrefix(v, "enc:v1:") {
			t.Fatalf("%s is not sealed", field)
		}
	}
	fi, err := os.Stat(masterKeyPath(meta))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("master.key: %v %v", fi, err)
	}
}

func TestMetadata_RoundTripPasswordAndKey(t *testing.T) {
	repo, _, _ := newSecretRepo(t)
	pw, key := genPassword(t), genPrivateKey(t)
	if err := repo.AddServer(domain.Server{Alias: "a", Host: "h", PrivateKey: key, Password: pw}); err != nil {
		t.Fatal(err)
	}
	s := onlyServer(t, repo)
	if s.Password != pw || s.PrivateKey != key {
		t.Fatal("secrets did not round trip")
	}
}

func TestMetadata_NoSecretsNoMasterKey(t *testing.T) {
	repo, _, meta := newSecretRepo(t)
	if err := repo.AddServer(domain.Server{Alias: "a", Host: "h", Tags: []string{"x"}}); err != nil {
		t.Fatal(err)
	}
	assertNoFile(t, masterKeyPath(meta))
}

func TestMetadata_LegacyPlaintextMigratedOnNextWrite(t *testing.T) {
	repo, cfg, meta := newSecretRepo(t)
	pw := genPassword(t)
	writeFile(t, cfg, "Host a\n  HostName h\n")
	legacy, err := json.Marshal(map[string]map[string]string{"a": {"password": pw}})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, meta, string(legacy))

	if s := onlyServer(t, repo); s.Password != pw {
		t.Fatal("legacy plaintext password not loaded")
	}
	if err := repo.SetPinned("a", true); err != nil {
		t.Fatal(err)
	}
	raw := readFile(t, meta)
	if !strings.Contains(raw, "enc:v1:") || strings.Contains(raw, pw) {
		t.Fatal("legacy password not migrated to sealed form")
	}
}

// tamperSealed flips one ciphertext byte of every sealed secret in metadata.json.
func tamperSealed(t *testing.T, meta string) {
	t.Helper()
	m := rawMetadata(t, meta)
	for _, entry := range m {
		for _, field := range []string{"private_key", "password"} {
			v, ok := entry[field].(string)
			if !ok {
				continue
			}
			raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(v, "enc:v1:"))
			if err != nil {
				t.Fatal(err)
			}
			raw[len(raw)-1] ^= 0xff
			entry[field] = "enc:v1:" + base64.StdEncoding.EncodeToString(raw)
		}
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, meta, string(data))
}

// failingOps runs read and write operations that must fail closed and returns their errors.
func failingOps(t *testing.T, repo *Repository, meta string) []error {
	t.Helper()
	before := readFile(t, meta)
	_, listErr := repo.ListServers("")
	if listErr == nil {
		t.Fatal("ListServers succeeded on undecryptable metadata")
	}
	pinErr := repo.SetPinned("a", true)
	if pinErr == nil {
		t.Fatal("SetPinned succeeded on undecryptable metadata")
	}
	_, pwErr := repo.GetPassword("a")
	if pwErr == nil {
		t.Fatal("GetPassword succeeded on undecryptable metadata")
	}
	if readFile(t, meta) != before {
		t.Fatal("metadata.json changed after failed load")
	}
	return []error{listErr, pinErr, pwErr}
}

func TestMetadata_TamperedValueFailsLoadWithoutWrite(t *testing.T) {
	repo, _, meta := newSecretRepo(t)
	if err := repo.AddServer(domain.Server{Alias: "a", Host: "h", Password: genPassword(t)}); err != nil {
		t.Fatal(err)
	}
	tamperSealed(t, meta)
	errs := failingOps(t, repo, meta)
	if msg := errs[0].Error(); !strings.Contains(msg, "metadata.json") || !strings.Contains(msg, `"a"`) {
		t.Fatalf("error does not name file and alias: %v", errs[0])
	}
}

func TestMetadata_MissingMasterKeyFailsWithoutWrite(t *testing.T) {
	repo, _, meta := newSecretRepo(t)
	if err := repo.AddServer(domain.Server{Alias: "a", Host: "h", Password: genPassword(t)}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(masterKeyPath(meta)); err != nil {
		t.Fatal(err)
	}
	errs := failingOps(t, repo, meta)
	if !strings.Contains(errs[0].Error(), "master.key") {
		t.Fatalf("error does not name master.key: %v", errs[0])
	}
	assertNoFile(t, masterKeyPath(meta))
}

func TestMetadata_ErrorsNeverContainSecret(t *testing.T) {
	repo, _, meta := newSecretRepo(t)
	pw, key := genPassword(t), genPrivateKey(t)
	if err := repo.AddServer(domain.Server{Alias: "a", Host: "h", PrivateKey: key, Password: pw}); err != nil {
		t.Fatal(err)
	}
	tamperSealed(t, meta)
	errs := failingOps(t, repo, meta)

	if err := os.Remove(masterKeyPath(meta)); err != nil {
		t.Fatal(err)
	}
	errs = append(errs, failingOps(t, repo, meta)...)

	for _, err := range errs {
		assertNoSecretIn(t, err.Error(), pw, key)
	}
}

func TestRepo_GetPassword(t *testing.T) {
	repo, _, _ := newSecretRepo(t)
	pw := genPassword(t)
	if err := repo.AddServer(domain.Server{Alias: "a", Host: "h", Password: pw}); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.GetPassword("a"); err != nil || got != pw {
		t.Fatalf("GetPassword(a) wrong (err=%v)", err)
	}

	// Legacy plaintext with an embedded key: lookups must not write anything (FR-015).
	legacyRepo, cfg, meta := newSecretRepo(t)
	writeFile(t, cfg, "Host a\n  HostName h\n")
	legacy, err := json.Marshal(map[string]map[string]string{"a": {"password": pw, "private_key": genPrivateKey(t)}})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, meta, string(legacy))

	if got, err := legacyRepo.GetPassword("a"); err != nil || got != pw {
		t.Fatalf("GetPassword(a) on legacy wrong (err=%v)", err)
	}
	if got, err := legacyRepo.GetPassword("missing"); err != nil || got != "" {
		t.Fatalf("GetPassword(missing) = non-empty or err=%v", err)
	}
	if readFile(t, meta) != string(legacy) {
		t.Fatal("GetPassword changed metadata.json")
	}
	assertNoFile(t, filepath.Join(filepath.Dir(meta), "keys"))
	assertNoFile(t, masterKeyPath(meta))
}

func TestRepo_RenameMovesPassword(t *testing.T) {
	repo, _, _ := newSecretRepo(t)
	pw := genPassword(t)
	old := domain.Server{Alias: "a", Host: "h", Password: pw}
	if err := repo.AddServer(old); err != nil {
		t.Fatal(err)
	}
	renamed := old
	renamed.Alias = "b"
	if err := repo.UpdateServer(old, renamed); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.GetPassword("b"); err != nil || got != pw {
		t.Fatalf("password not under new alias (err=%v)", err)
	}
	if got, err := repo.GetPassword("a"); err != nil || got != "" {
		t.Fatalf("password still under old alias (err=%v)", err)
	}
}
