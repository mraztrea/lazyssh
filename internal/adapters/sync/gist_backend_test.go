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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/ports"
)

type gistBody struct {
	Description string `json:"description"`
	Public      *bool  `json:"public"`
	Files       map[string]struct {
		Content string `json:"content"`
	} `json:"files"`
}

func newTestGist(t *testing.T, gistID string, h http.HandlerFunc) ports.SyncBackend {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return newGistBackend("tok", gistID, srv.URL, srv.Client())
}

func decodeGistBody(t *testing.T, r *http.Request) gistBody {
	t.Helper()
	var body gistBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Errorf("decode body: %v", err)
	}
	return body
}

func TestGist_UploadCreatesWhenNoID(t *testing.T) {
	blob := []byte(`{"v":1,"data":"abc"}`)
	var called bool
	b := newTestGist(t, "", func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/gists" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth header %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("accept header %q", r.Header.Get("Accept"))
		}
		body := decodeGistBody(t, r)
		if body.Public == nil || *body.Public || body.Description != "lazyssh sync" {
			t.Errorf("body %+v", body)
		}
		if body.Files[gistFileName].Content != string(blob) {
			t.Errorf("content %q", body.Files[gistFileName].Content)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"g1"}`))
	})
	id, err := b.Upload(context.Background(), blob)
	if err != nil || id != "g1" || !called {
		t.Fatal(id, err, called)
	}
}

func TestGist_UploadPatchesWhenID(t *testing.T) {
	b := newTestGist(t, "g1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/gists/g1" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		if decodeGistBody(t, r).Files[gistFileName].Content != "blob" {
			t.Error("content not sent")
		}
		_, _ = w.Write([]byte(`{"id":"g1"}`))
	})
	id, err := b.Upload(context.Background(), []byte("blob"))
	if err != nil || id != "g1" {
		t.Fatal(id, err)
	}
}

func TestGist_DownloadReadsContent(t *testing.T) {
	b := newTestGist(t, "g1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/gists/g1" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"g1","files":{"lazyssh-sync.enc":{"content":"BLOB","truncated":false}}}`))
	})
	got, err := b.Download(context.Background())
	if err != nil || string(got) != "BLOB" {
		t.Fatal(string(got), err)
	}
}

func TestGist_DownloadFollowsRawURLWhenTruncated(t *testing.T) {
	var srvURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/gists/g1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"files": map[string]any{gistFileName: map[string]any{
			"content": "PART", "truncated": true, "raw_url": srvURL + "/raw",
		}}})
	})
	mux.HandleFunc("/raw", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("token sent to raw_url")
		}
		_, _ = w.Write([]byte("FULL-BLOB"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	srvURL = srv.URL

	got, err := newGistBackend("tok", "g1", srv.URL, srv.Client()).Download(context.Background())
	if err != nil || string(got) != "FULL-BLOB" {
		t.Fatal(string(got), err)
	}
}

func TestGist_Non2xxIncludesMessage(t *testing.T) {
	b := newTestGist(t, "g1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	})
	_, err := b.Download(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Bad credentials") || !strings.Contains(err.Error(), "401") ||
		strings.Contains(err.Error(), "tok") {
		t.Fatalf("got %v", err)
	}
}

func TestGist_DownloadWithoutIDOrFileIsErrNoRemote(t *testing.T) {
	if _, err := newGistBackend("tok", "", "http://127.0.0.1:0", http.DefaultClient).Download(context.Background()); !errors.Is(err, ports.ErrNoRemote) {
		t.Fatalf("empty id: got %v", err)
	}
	b := newTestGist(t, "g1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"files":{}}`))
	})
	if _, err := b.Download(context.Background()); !errors.Is(err, ports.ErrNoRemote) {
		t.Fatalf("missing file: got %v", err)
	}
}
