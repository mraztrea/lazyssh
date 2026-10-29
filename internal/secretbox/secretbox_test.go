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

package secretbox

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func randomKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func randomSecret(t *testing.T) string {
	t.Helper()
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func assertNoSecret(t *testing.T, err error, secret string) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), secret) {
		t.Fatal("error message contains the secret")
	}
}

func TestSealOpen_RoundTrip(t *testing.T) {
	key, plain := randomKey(t), randomSecret(t)
	sealed, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(key, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatal("round trip mismatch")
	}
	if !IsSealed(sealed) || IsSealed(plain) {
		t.Fatal("IsSealed wrong")
	}
}

func TestSeal_FreshNonceEachTime(t *testing.T) {
	key, plain := randomKey(t), randomSecret(t)
	a, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two seals of the same value are identical")
	}
}

func TestSeal_NoPlaintextInOutput(t *testing.T) {
	key, plain := randomKey(t), randomSecret(t)
	sealed, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sealed, "enc:v1:") {
		t.Fatalf("missing prefix: %q", sealed[:7])
	}
	if strings.Contains(sealed, plain) {
		t.Fatal("sealed output contains plaintext")
	}
}

func TestOpen_TamperedFails(t *testing.T) {
	key, plain := randomKey(t), randomSecret(t)
	sealed, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, "enc:v1:"))
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xff
	tampered := "enc:v1:" + base64.StdEncoding.EncodeToString(raw)
	_, err = Open(key, tampered)
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("want ErrDecrypt, got %v", err)
	}
	assertNoSecret(t, err, plain)
}

func TestOpen_WrongKeyFails(t *testing.T) {
	plain := randomSecret(t)
	sealed, err := Seal(randomKey(t), plain)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Open(randomKey(t), sealed)
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("want ErrDecrypt, got %v", err)
	}
	assertNoSecret(t, err, plain)
}

func TestOpen_LegacyPlaintextPassthrough(t *testing.T) {
	plain := randomSecret(t)
	got, err := Open(nil, plain)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatal("legacy plaintext not returned as-is")
	}
}

func TestOpen_UnknownVersion(t *testing.T) {
	_, err := Open(randomKey(t), "enc:v9:AAAA")
	if !errors.Is(err, ErrUnknownFormat) {
		t.Fatalf("want ErrUnknownFormat, got %v", err)
	}
}

func TestLoadOrCreateKey_Creates0600In0700Dir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub")
	path := filepath.Join(dir, "master.key")
	key, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("key length %d", len(key))
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %o", fi.Mode().Perm())
	}
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %o", di.Mode().Perm())
	}
}

func TestLoadOrCreateKey_KeepsExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "master.key")
	first, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("existing key was replaced")
	}

	short := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(short, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadOrCreateKey(short)
	if err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("want corrupt error, got %v", err)
	}
	data, err := os.ReadFile(short)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "12345" {
		t.Fatal("corrupt key file was overwritten")
	}
}

func TestLoadKey_MissingNamesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "master.key")
	_, err := LoadKey(path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("error does not name path: %v", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want fs.ErrNotExist, got %v", err)
	}
}

func TestLoadKey_WrongSizeIsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(path, make([]byte, 31), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadKey(path)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("want corrupt error naming path, got %v", err)
	}
}
