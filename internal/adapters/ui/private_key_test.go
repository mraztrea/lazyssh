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

package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrivateKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "k")
	if err := os.WriteFile(p, []byte("KEY\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := resolvePrivateKey("pasted", p); got != "pasted" {
		t.Fatal("text must win")
	}
	if got, _ := resolvePrivateKey("", p); got != "KEY\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := resolvePrivateKey("", filepath.Join(dir, "missing")); err == nil {
		t.Fatal("want error")
	}
	if got, _ := resolvePrivateKey("", ""); got != "" {
		t.Fatal("want empty")
	}
	t.Setenv("HOME", dir)
	if got, _ := resolvePrivateKey("", "~/k"); got != "KEY\n" {
		t.Fatalf("tilde not expanded: got %q", got)
	}
}
