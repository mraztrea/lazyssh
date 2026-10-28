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

package sync

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/ports"
)

func TestFileBackend_RoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "lazyssh-sync.enc")
	b := NewFileBackend(p)
	blob := []byte(`{"v":1}`)

	id, err := b.Upload(context.Background(), blob)
	if err != nil || id != p {
		t.Fatal(id, err)
	}
	got, err := b.Download(context.Background())
	if err != nil || !bytes.Equal(got, blob) {
		t.Fatal(string(got), err)
	}
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatal(fi, err)
	}
}

func TestFileBackend_MissingIsErrNoRemote(t *testing.T) {
	b := NewFileBackend(filepath.Join(t.TempDir(), "missing.enc"))
	if _, err := b.Download(context.Background()); !errors.Is(err, ports.ErrNoRemote) {
		t.Fatalf("got %v", err)
	}
}

func TestFileBackend_ExpandsTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	b := NewFileBackend("~/sync.enc")
	id, err := b.Upload(context.Background(), []byte("x"))
	if err != nil || id != filepath.Join(home, "sync.enc") {
		t.Fatal(id, err)
	}
}
