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

// Package secretbox seals single string values with AES-256-GCM under a per-machine key file.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	prefix  = "enc:v1:"
	keySize = 32
)

var (
	ErrDecrypt       = errors.New("cannot decrypt value")
	ErrUnknownFormat = errors.New("unknown encrypted value format")
)

// LoadKey reads an existing master key file.
func LoadKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("master key %s: %w", path, err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("master key %s is corrupt", path)
	}
	return key, nil
}

// LoadOrCreateKey reads the master key, creating it (0600 in a 0700 dir) if missing. It never overwrites.
func LoadOrCreateKey(path string) ([]byte, error) {
	key, err := LoadKey(path)
	if !errors.Is(err, fs.ErrNotExist) {
		return key, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("master key %s: %w", path, err)
	}
	key = make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("master key %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return LoadKey(path) // another writer created it first
	}
	if err != nil {
		return nil, fmt.Errorf("master key %s: %w", path, err)
	}
	_, werr := f.Write(key)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return nil, fmt.Errorf("master key %s: %w", path, werr)
	}
	return key, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Seal encrypts plain into "enc:v1:" + base64(nonce || ciphertext).
func Seal(key []byte, plain string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return prefix + base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plain), nil)), nil
}

// Open decrypts a sealed value; values without the "enc:" prefix are legacy plaintext and returned as-is.
func Open(key []byte, v string) (string, error) {
	if !IsSealed(v) {
		return v, nil
	}
	if !strings.HasPrefix(v, prefix) {
		return "", ErrUnknownFormat
	}
	raw, err := base64.StdEncoding.DecodeString(v[len(prefix):])
	if err != nil {
		return "", ErrDecrypt
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", ErrDecrypt
	}
	if len(raw) < gcm.NonceSize()+gcm.Overhead() {
		return "", ErrDecrypt
	}
	n := gcm.NonceSize()
	plain, err := gcm.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return "", ErrDecrypt
	}
	return string(plain), nil
}

// IsSealed reports whether v carries the encrypted-value prefix.
func IsSealed(v string) bool {
	return strings.HasPrefix(v, "enc:")
}
