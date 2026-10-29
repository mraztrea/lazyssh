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
	"errors"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

// Auth methods offered by the form; derived from the stored secrets, never stored themselves.
const (
	authIdentityFile = "identity file"
	authEmbeddedKey  = "embedded key"
	authPassword     = "password"
)

var authMethods = []string{authIdentityFile, authEmbeddedKey, authPassword}

// authMethodOf derives the auth method from which secret a server holds (password wins).
func authMethodOf(s domain.Server) string {
	switch {
	case s.Password != "":
		return authPassword
	case s.PrivateKey != "":
		return authEmbeddedKey
	default:
		return authIdentityFile
	}
}

// applyAuthMethod clears the secrets of the methods not chosen, so at most one is saved.
func applyAuthMethod(d *ServerFormData) error {
	switch d.AuthMethod {
	case authPassword:
		if d.Password == "" {
			return errors.New("password is required")
		}
		d.PrivateKey, d.PrivateKeyImportPath = "", ""
	case authEmbeddedKey:
		d.Password = ""
	default:
		d.Password, d.PrivateKey, d.PrivateKeyImportPath = "", "", ""
	}
	return nil
}

// authFieldsEnabled reports which secret inputs the form enables for method.
func authFieldsEnabled(method string) (key, password bool) {
	return method == authEmbeddedKey, method == authPassword
}
