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
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

func TestAuthMethodOf(t *testing.T) {
	cases := []struct {
		s    domain.Server
		want string
	}{
		{domain.Server{Password: "p", PrivateKey: "k"}, authPassword},
		{domain.Server{PrivateKey: "k"}, authEmbeddedKey},
		{domain.Server{}, authIdentityFile},
	}
	for _, c := range cases {
		if got := authMethodOf(c.s); got != c.want {
			t.Errorf("authMethodOf = %q, want %q", got, c.want)
		}
	}
}

func TestApplyAuthMethod_PasswordClearsKey(t *testing.T) {
	d := ServerFormData{AuthMethod: authPassword, Password: " pw ", PrivateKey: "k", PrivateKeyImportPath: "/p"}
	if err := applyAuthMethod(&d); err != nil {
		t.Fatal(err)
	}
	if d.PrivateKey != "" || d.PrivateKeyImportPath != "" || d.Password != " pw " {
		t.Fatalf("got %+v", d)
	}
}

func TestApplyAuthMethod_PasswordRequired(t *testing.T) {
	d := ServerFormData{AuthMethod: authPassword}
	if err := applyAuthMethod(&d); err == nil || err.Error() != "password is required" {
		t.Fatalf("got %v", err)
	}
}

func TestApplyAuthMethod_EmbeddedKeyClearsPassword(t *testing.T) {
	d := ServerFormData{AuthMethod: authEmbeddedKey, Password: "pw", PrivateKey: "k", PrivateKeyImportPath: "/p"}
	if err := applyAuthMethod(&d); err != nil {
		t.Fatal(err)
	}
	if d.Password != "" || d.PrivateKey != "k" || d.PrivateKeyImportPath != "/p" {
		t.Fatalf("got %+v", d)
	}
}

func TestApplyAuthMethod_IdentityFileClearsBoth(t *testing.T) {
	d := ServerFormData{AuthMethod: authIdentityFile, Password: "pw", PrivateKey: "k", PrivateKeyImportPath: "/p"}
	if err := applyAuthMethod(&d); err != nil {
		t.Fatal(err)
	}
	if d.Password != "" || d.PrivateKey != "" || d.PrivateKeyImportPath != "" {
		t.Fatalf("got %+v", d)
	}
}

func TestAuthFieldsEnabled(t *testing.T) {
	cases := []struct {
		method        string
		key, password bool
	}{
		{authIdentityFile, false, false},
		{authEmbeddedKey, true, false},
		{authPassword, false, true},
	}
	for _, c := range cases {
		if key, pw := authFieldsEnabled(c.method); key != c.key || pw != c.password {
			t.Errorf("%s: key=%v password=%v", c.method, key, pw)
		}
	}
}
