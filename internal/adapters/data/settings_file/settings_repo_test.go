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
	"os"
	"path/filepath"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

func TestSettingsRepo_LoadMissingReturnsZero(t *testing.T) {
	r := NewRepository(filepath.Join(t.TempDir(), "settings.json"))
	s, err := r.Load()
	if err != nil || s != (domain.Settings{}) {
		t.Fatal(s, err)
	}
}

func TestSettingsRepo_RoundTripAndPerms(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "settings.json")
	r := NewRepository(p)
	in := domain.Settings{Editor: "nvim", Sync: domain.SyncSettings{Provider: "gist", GistID: "abc", GistToken: "tok"}}
	if err := r.Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := r.Load()
	if err != nil || out != in {
		t.Fatalf("%+v %v", out, err)
	}
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatal(fi, err)
	}
	di, err := os.Stat(filepath.Dir(p))
	if err != nil || di.Mode().Perm() != 0o700 {
		t.Fatal(di, err)
	}
}
