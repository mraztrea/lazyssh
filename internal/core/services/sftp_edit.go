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

import "strings"

// EditorCommand returns the editor argv for an $EDITOR value ("code --wait" splits on spaces),
// defaulting to vi. The file to edit is appended by the caller.
func EditorCommand(env string) []string {
	if f := strings.Fields(env); len(f) > 0 {
		return f
	}
	return []string{"vi"}
}
