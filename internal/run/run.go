// Package run abstracts external command execution so jj/tmux
// interactions can be faked in tests.
package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
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

// HookRunner executes a user-configured lifecycle hook command, streaming
// its output live and enforcing an optional timeout.
type HookRunner interface {
	RunHook(dir, command string, env []string, timeout time.Duration) error
}

// ExecHookRunner runs hook commands with os/exec via "sh -c". Stdout/Stderr
// default to os.Stdout/os.Stderr when nil, so the zero value keeps today's
// behavior; callers that need to keep hook output off the real stdout/stderr
// (e.g. the sidebar's alt-screen) can set them explicitly.
type ExecHookRunner struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (r ExecHookRunner) RunHook(dir, command string, env []string, timeout time.Duration) error {
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	if r.Stdout != nil {
		cmd.Stdout = r.Stdout
	}
	cmd.Stderr = os.Stderr
	if r.Stderr != nil {
		cmd.Stderr = r.Stderr
	}
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("hook command %q timed out after %s", command, timeout)
	}
	if err != nil {
		return fmt.Errorf("hook command %q: %w", command, err)
	}
	return nil
}
