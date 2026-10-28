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
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"go.uber.org/zap"
)

type memSettings struct{ s domain.Settings }

func (m *memSettings) Load() (domain.Settings, error) { return m.s, nil }
func (m *memSettings) Save(s domain.Settings) error   { m.s = s; return nil }

type fakeSnapshot struct {
	bundle   domain.SyncBundle
	imported bool
}

func (f *fakeSnapshot) Export() (domain.SyncBundle, error) { return f.bundle, nil }
func (f *fakeSnapshot) Import(b domain.SyncBundle) error {
	f.bundle, f.imported = b, true
	return nil
}

type memBackend struct {
	blob []byte
	id   string
}

func (m *memBackend) Upload(_ context.Context, blob []byte) (string, error) {
	m.blob = blob
	return m.id, nil
}

func (m *memBackend) Download(_ context.Context) ([]byte, error) {
	if m.blob == nil {
		return nil, ports.ErrNoRemote
	}
	return m.blob, nil
}

func newTestSync(t *testing.T, provider string, snap *fakeSnapshot, backend *memBackend) (ports.SyncService, *memSettings, *int) {
	t.Helper()
	fastKDF(t)
	settings := &memSettings{s: domain.Settings{Sync: domain.SyncSettings{Provider: provider}}}
	calls := new(int)
	backendFor := func(domain.SyncSettings) (ports.SyncBackend, error) {
		*calls++
		return backend, nil
	}
	return NewSyncService(zap.NewNop().Sugar(), settings, snap, backendFor), settings, calls
}

func TestSync_PushThenPull_RoundTrip(t *testing.T) {
	backend := &memBackend{id: "/tmp/x"}
	a := &fakeSnapshot{bundle: domain.SyncBundle{Version: 1, SSHConfig: "Host a\n", Metadata: json.RawMessage(`{}`)}}
	pusher, pushSettings, _ := newTestSync(t, "file", a, backend)
	if err := pusher.Push(context.Background(), "pw"); err != nil {
		t.Fatal(err)
	}

	b := &fakeSnapshot{}
	puller, pullSettings, _ := newTestSync(t, "file", b, backend)
	if err := puller.Pull(context.Background(), "pw"); err != nil {
		t.Fatal(err)
	}
	if !b.imported || b.bundle.SSHConfig != "Host a\n" {
		t.Fatalf("imported %+v", b)
	}
	if pushSettings.s.Sync.LastPushAt.IsZero() || pullSettings.s.Sync.LastPullAt.IsZero() {
		t.Fatal("timestamps not saved")
	}
}

func TestSync_PullWrongPassword_DoesNotImport(t *testing.T) {
	backend := &memBackend{}
	pusher, _, _ := newTestSync(t, "file", &fakeSnapshot{bundle: domain.SyncBundle{Version: 1, Metadata: json.RawMessage(`{}`)}}, backend)
	if err := pusher.Push(context.Background(), "right"); err != nil {
		t.Fatal(err)
	}
	snap := &fakeSnapshot{}
	puller, settings, _ := newTestSync(t, "file", snap, backend)
	err := puller.Pull(context.Background(), "wrong")
	if !errors.Is(err, ErrBadPassword) {
		t.Fatalf("got %v", err)
	}
	if snap.imported || !settings.s.Sync.LastPullAt.IsZero() {
		t.Fatal("wrong password touched local state")
	}
}

func TestSync_NotConfigured(t *testing.T) {
	svc, _, calls := newTestSync(t, "", &fakeSnapshot{}, &memBackend{})
	if err := svc.Push(context.Background(), "pw"); !errors.Is(err, ErrSyncNotConfigured) {
		t.Fatalf("push: got %v", err)
	}
	if err := svc.Pull(context.Background(), "pw"); !errors.Is(err, ErrSyncNotConfigured) {
		t.Fatalf("pull: got %v", err)
	}
	if *calls != 0 {
		t.Fatal("backendFor called without a provider")
	}
}

func TestSync_EmptyPasswordTouchesNothing(t *testing.T) {
	snap := &fakeSnapshot{}
	svc, _, calls := newTestSync(t, "file", snap, &memBackend{blob: []byte("x")})
	if err := svc.Push(context.Background(), ""); !errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("push: got %v", err)
	}
	if err := svc.Pull(context.Background(), ""); !errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("pull: got %v", err)
	}
	if *calls != 0 || snap.imported {
		t.Fatal("empty password reached the backend")
	}
}

func TestSync_PushSavesReturnedGistID(t *testing.T) {
	svc, settings, _ := newTestSync(t, "gist", &fakeSnapshot{bundle: domain.SyncBundle{Version: 1, Metadata: json.RawMessage(`{}`)}}, &memBackend{id: "g1"})
	if err := svc.Push(context.Background(), "pw"); err != nil {
		t.Fatal(err)
	}
	if settings.s.Sync.GistID != "g1" {
		t.Fatalf("gist id %q", settings.s.Sync.GistID)
	}
}
