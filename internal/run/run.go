// Package run abstracts external command execution so jj/tmux
// interactions can be faked in tests.
package run

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes an external command in dir and returns its trimmed stdout.
// Stderr is folded into the returned error.
type Runner interface {
	Run(dir, name string, args ...string) (string, error)
}

// ExecRunner runs commands with os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s",
			name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
