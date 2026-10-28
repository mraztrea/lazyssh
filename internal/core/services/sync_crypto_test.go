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

package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

// fastKDF keeps the suite quick; the production value is checked in TestSeal_NoPlaintextLeak.
func fastKDF(t *testing.T) {
	t.Helper()
	old := pbkdf2Iterations
	pbkdf2Iterations = 1000
	t.Cleanup(func() { pbkdf2Iterations = old })
}

func testBundle() domain.SyncBundle {
	return domain.SyncBundle{Version: 1, SSHConfig: "Host a\n", Metadata: json.RawMessage(`{"a":{}}`)}
}

func TestSealOpen_RoundTrip(t *testing.T) {
	fastKDF(t)
	in := testBundle()
	blob, err := sealBundle(in, "pw")
	if err != nil {
		t.Fatal(err)
	}
	out, err := openBundle(blob, "pw")
	if err != nil || out.SSHConfig != in.SSHConfig || string(out.Metadata) != string(in.Metadata) {
		t.Fatal(err, out)
	}
}

func TestOpen_WrongPassword(t *testing.T) {
	fastKDF(t)
	blob, err := sealBundle(testBundle(), "pw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openBundle(blob, "nope"); !errors.Is(err, ErrBadPassword) {
		t.Fatalf("got %v", err)
	}
}

func TestSeal_EmptyPassword(t *testing.T) {
	if _, err := sealBundle(testBundle(), ""); !errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("seal: got %v", err)
	}
	if _, err := openBundle([]byte(`{}`), ""); !errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("open: got %v", err)
	}
}

func TestOpen_TamperedCiphertext(t *testing.T) {
	fastKDF(t)
	blob, err := sealBundle(testBundle(), "pw")
	if err != nil {
		t.Fatal(err)
	}
	var env envelope
	if err := json.Unmarshal(blob, &env); err != nil {
		t.Fatal(err)
	}
	env.Data[0] ^= 0x01
	tampered, _ := json.Marshal(env)
	if _, err := openBundle(tampered, "pw"); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}

func TestSeal_NoPlaintextLeak(t *testing.T) {
	blob, err := sealBundle(testBundle(), "pw") // production iteration count
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte("Host a")) {
		t.Fatal("plaintext in blob")
	}
	var env map[string]any
	if err := json.Unmarshal(blob, &env); err != nil {
		t.Fatal(err)
	}
	var e envelope
	_ = json.Unmarshal(blob, &e)
	if env["v"] != float64(1) || env["kdf"] != "pbkdf2-sha256" || env["iter"] != float64(600000) ||
		len(e.Salt) != 16 || len(e.Nonce) != 12 || len(e.Data) == 0 {
		t.Fatalf("bad envelope: %s", blob)
	}
}
