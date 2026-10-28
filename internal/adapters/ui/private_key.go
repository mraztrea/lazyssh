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
	"strings"
)

// resolvePrivateKey returns the pasted key text if any, otherwise the content of importPath
// (leading ~ expanded), otherwise "".
func resolvePrivateKey(text, importPath string) (string, error) {
	if strings.TrimSpace(text) != "" {
		return text, nil
	}
	importPath = strings.TrimSpace(importPath)
	if importPath == "" {
		return "", nil
	}
	if importPath == "~" || strings.HasPrefix(importPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		importPath = filepath.Join(home, importPath[1:])
	}
	data, err := os.ReadFile(filepath.Clean(importPath)) // user-chosen path is the point of import
	if err != nil {
		return "", err
	}
	return string(data), nil
}
