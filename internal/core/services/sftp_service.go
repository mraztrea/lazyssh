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
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/pkg/sftp"
	"go.uber.org/zap"
)

const stderrTailSize = 4 << 10

// ponytail: subprocess transport. Upgrade to golang.org/x/crypto/ssh only if an in-process client is ever required.
type sftpService struct {
	logger *zap.SugaredLogger
	cmdFor func(alias string) *exec.Cmd
}

// NewSFTPService opens SFTP sessions by running `ssh -s sftp <alias>`, so the user's SSH config applies.
func NewSFTPService(logger *zap.SugaredLogger) ports.SFTPService {
	return newSFTPServiceWithCmd(logger, func(alias string) *exec.Cmd {
		return exec.Command("ssh", "-s", "sftp", alias)
	})
}

func newSFTPServiceWithCmd(logger *zap.SugaredLogger, cmdFor func(alias string) *exec.Cmd) ports.SFTPService {
	return &sftpService{logger: logger, cmdFor: cmdFor}
}

func (s *sftpService) Connect(alias string) (ports.SFTPSession, error) {
	cmd := s.cmdFor(alias)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &tailBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sftp %s: %w", alias, err)
	}
	// If ssh exits (auth failure, unknown host) stdout hits EOF and the handshake fails at once.
	client, err := sftp.NewClientPipe(stdout, stdin)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait() // also waits for the stderr copy to finish
		s.logger.Errorw("sftp connect failed", "alias", alias, "error", err, "stderr", stderr.String())
		if line := stderr.lastLine(); line != "" {
			return nil, fmt.Errorf("sftp %s: %w: %s", alias, err, line)
		}
		return nil, fmt.Errorf("sftp %s: %w", alias, err)
	}
	s.logger.Infow("sftp session opened", "alias", alias)
	return &sftpSession{logger: s.logger, alias: alias, client: client, cmd: cmd, stdin: stdin, stderr: stderr}, nil
}

type sftpSession struct {
	logger *zap.SugaredLogger
	alias  string
	client *sftp.Client
	cmd    *exec.Cmd
	stdin  io.Closer
	stderr *tailBuffer
	once   sync.Once
	err    error
}

func (s *sftpSession) Client() *sftp.Client { return s.client }

// Close closes the client and kills the ssh subprocess; safe to call more than once.
func (s *sftpSession) Close() error {
	s.once.Do(func() {
		s.err = s.client.Close()
		_ = s.stdin.Close()
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait() // "signal: killed" is expected
		if tail := s.stderr.String(); tail != "" {
			s.logger.Debugw("sftp ssh stderr", "alias", s.alias, "stderr", tail)
		}
		s.logger.Infow("sftp session closed", "alias", s.alias)
	})
	return s.err
}

// tailBuffer keeps the last stderrTailSize bytes written to it.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - stderrTailSize; over > 0 {
		b.buf = b.buf[over:]
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

func (b *tailBuffer) lastLine() string {
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
