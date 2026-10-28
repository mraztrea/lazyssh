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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Adembc/lazyssh/internal/core/ports"
)

const (
	gistFileName    = "lazyssh-sync.enc"
	gistDescription = "lazyssh sync"
	githubAPI       = "https://api.github.com"
)

type gistBackend struct {
	token   string
	gistID  string
	baseURL string
	client  *http.Client
}

type gistFile struct {
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
	RawURL    string `json:"raw_url,omitempty"`
}

type gist struct {
	ID          string              `json:"id,omitempty"`
	Description string              `json:"description,omitempty"`
	Public      *bool               `json:"public,omitempty"`
	Files       map[string]gistFile `json:"files"`
}

// NewGistBackend stores the blob in a secret GitHub gist. An empty gistID creates a new gist on upload.
func NewGistBackend(token, gistID string) ports.SyncBackend {
	return newGistBackend(token, gistID, githubAPI, &http.Client{Timeout: 30 * time.Second})
}

func newGistBackend(token, gistID, baseURL string, client *http.Client) *gistBackend {
	return &gistBackend{token: token, gistID: gistID, baseURL: baseURL, client: client}
}

func (g *gistBackend) Upload(ctx context.Context, blob []byte) (string, error) {
	body := gist{Files: map[string]gistFile{gistFileName: {Content: string(blob)}}}
	method, url := http.MethodPatch, g.baseURL+"/gists/"+g.gistID
	if g.gistID == "" {
		public := false
		body.Description, body.Public = gistDescription, &public
		method, url = http.MethodPost, g.baseURL+"/gists"
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	var resp gist
	if err := g.do(ctx, method, url, payload, true, &resp); err != nil {
		return "", err
	}
	if resp.ID == "" {
		return "", fmt.Errorf("github: response has no gist id")
	}
	return resp.ID, nil
}

func (g *gistBackend) Download(ctx context.Context) ([]byte, error) {
	if g.gistID == "" {
		return nil, fmt.Errorf("%w: no gist id configured", ports.ErrNoRemote)
	}
	var resp gist
	if err := g.do(ctx, http.MethodGet, g.baseURL+"/gists/"+g.gistID, nil, true, &resp); err != nil {
		return nil, err
	}
	f, ok := resp.Files[gistFileName]
	if !ok {
		return nil, fmt.Errorf("%w: gist %s has no %s", ports.ErrNoRemote, g.gistID, gistFileName)
	}
	if !f.Truncated {
		return []byte(f.Content), nil
	}
	// Large files are truncated inline; raw_url is a capability URL, so the token is not sent there.
	var raw bytes.Buffer
	if err := g.do(ctx, http.MethodGet, f.RawURL, nil, false, &raw); err != nil {
		return nil, err
	}
	return raw.Bytes(), nil
}

// do sends a request and decodes a 2xx body into out (*bytes.Buffer receives it raw).
// Errors carry the HTTP status and GitHub's message, never the token or the payload.
func (g *gistBackend) do(ctx context.Context, method, url string, payload []byte, auth bool, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+g.token)
		req.Header.Set("Accept", "application/vnd.github+json")
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("github: %s request failed: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&e)
		return fmt.Errorf("github: %s: %s", resp.Status, e.Message)
	}
	if buf, ok := out.(*bytes.Buffer); ok {
		_, err = buf.ReadFrom(resp.Body)
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("github: decode response: %w", err)
	}
	return nil
}
