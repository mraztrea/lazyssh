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
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

var (
	ErrPasswordRequired = errors.New("password is required")
	ErrBadPassword      = errors.New("wrong password or corrupted sync data")
)

const (
	envelopeVersion = 1
	envelopeKDF     = "pbkdf2-sha256"
	saltSize        = 16
	nonceSize       = 12
	keySize         = 32
	// maxIterations bounds the work an untrusted envelope can ask for.
	maxIterations = 10_000_000
)

// pbkdf2Iterations is a var so tests can lower it.
var pbkdf2Iterations = 600000

// envelope is the on-wire form of an encrypted bundle. []byte fields are base64 in JSON.
type envelope struct {
	V     int    `json:"v"`
	KDF   string `json:"kdf"`
	Iter  int    `json:"iter"`
	Salt  []byte `json:"salt"`
	Nonce []byte `json:"nonce"`
	Data  []byte `json:"data"`
}

func newGCM(password string, salt []byte, iter int) (cipher.AEAD, error) {
	key, err := pbkdf2.Key(sha256.New, password, salt, iter, keySize)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealBundle encrypts b as AES-256-GCM(gzip(json(b))) with a PBKDF2-SHA256 key.
func sealBundle(b domain.SyncBundle, password string) ([]byte, error) {
	if password == "" {
		return nil, ErrPasswordRequired
	}
	plain, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("marshal bundle: %w", err)
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(plain); err != nil {
		return nil, fmt.Errorf("compress bundle: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("compress bundle: %w", err)
	}

	env := envelope{
		V: envelopeVersion, KDF: envelopeKDF, Iter: pbkdf2Iterations,
		Salt: make([]byte, saltSize), Nonce: make([]byte, nonceSize),
	}
	if _, err := rand.Read(env.Salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(env.Nonce); err != nil {
		return nil, err
	}
	aead, err := newGCM(password, env.Salt, env.Iter)
	if err != nil {
		return nil, err
	}
	env.Data = aead.Seal(nil, env.Nonce, gz.Bytes(), nil)
	return json.Marshal(env)
}

// openBundle decrypts a blob produced by sealBundle. A wrong password or modified data yields ErrBadPassword.
func openBundle(blob []byte, password string) (domain.SyncBundle, error) {
	var b domain.SyncBundle
	if password == "" {
		return b, ErrPasswordRequired
	}
	var env envelope
	if err := json.Unmarshal(blob, &env); err != nil {
		return b, fmt.Errorf("sync data is not a lazyssh envelope: %w", err)
	}
	if env.V != envelopeVersion || env.KDF != envelopeKDF || env.Iter < 1 || env.Iter > maxIterations ||
		len(env.Salt) != saltSize || len(env.Nonce) != nonceSize {
		return b, errors.New("unsupported sync envelope format")
	}
	aead, err := newGCM(password, env.Salt, env.Iter)
	if err != nil {
		return b, err
	}
	gz, err := aead.Open(nil, env.Nonce, env.Data, nil)
	if err != nil {
		return b, ErrBadPassword
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return b, fmt.Errorf("decompress bundle: %w", err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		return b, fmt.Errorf("decompress bundle: %w", err)
	}
	if err := json.Unmarshal(plain, &b); err != nil {
		return b, fmt.Errorf("parse bundle: %w", err)
	}
	return b, nil
}
