package app

import (
	"fmt"
	"time"

	"github.com/richardcase/jumux/internal/run"
)

// hookEnv builds the JUMUX_* environment variables passed to every
// lifecycle hook command, appended to the subprocess's inherited
// environment. windowName is omitted when not yet known (e.g. Remove
// before a tmux window lookup has resolved one).
func hookEnv(event, feature, wsPath, repoRoot, windowName string) []string {
	env := []string{
		"JUMUX_EVENT=" + event,
		"JUMUX_FEATURE=" + feature,
		"JUMUX_WORKSPACE_PATH=" + wsPath,
		"JUMUX_REPO_ROOT=" + repoRoot,
	}
	if windowName != "" {
		env = append(env, "JUMUX_WINDOW_NAME="+windowName)
	}
	return env
}

// runHooks runs commands in order via runner, in dir, with env appended to
// each subprocess's environment and timeout applied to each command
// individually. It stops and returns a wrapped error at the first failure;
// an empty commands list is a no-op that never touches runner.
func runHooks(runner run.HookRunner, dir string, commands []string, timeout time.Duration, env []string) error {
	for _, cmd := range commands {
		if err := runner.RunHook(dir, cmd, env, timeout); err != nil {
			return fmt.Errorf("lifecycle hook %q: %w", cmd, err)
		}
	}
	return nil
}
