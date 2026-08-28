// Package notify sends OS desktop notifications. It knows nothing about jj
// or tmux: callers pass a command runner, so notifications are testable
// without shelling out for real.
package notify

import (
	"fmt"
	"runtime"

	"github.com/richardcase/agentmux/internal/run"
)

// Send delivers a desktop notification with title and message via the
// platform's notifier: osascript on macOS, notify-send on Linux. On any
// other platform Send is a silent no-op.
func Send(runner run.Runner, title, message string) error {
	return send(runner, runtime.GOOS, title, message)
}

func send(runner run.Runner, goos, title, message string) error {
	switch goos {
	case "darwin":
		script := fmt.Sprintf("display notification %q with title %q", message, title)
		_, err := runner.Run("", "osascript", "-e", script)
		return err
	case "linux":
		_, err := runner.Run("", "notify-send", title, message)
		return err
	default:
		return nil
	}
}
