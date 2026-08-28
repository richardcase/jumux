// Package tmuxctl wraps the tmux CLI operations jumux needs.
// Windows are targeted by ID (@N) since names are not unique; the custom
// window option @jumux-feature maps windows to features across renames.
package tmuxctl

import (
	"strings"

	"github.com/richardcase/jumux/internal/run"
)

// FeatureOption is the tmux window option that records which feature a
// window belongs to.
const FeatureOption = "@jumux-feature"

// Window is one row from list-windows.
type Window struct {
	ID      string
	Name    string
	Feature string // value of @jumux-feature, "" if unset
}

// NewWindow creates a detached window and returns its window ID.
func NewWindow(r run.Runner, name, dir string) (string, error) {
	return r.Run("", "tmux", "new-window", "-d", "-P", "-F", "#{window_id}",
		"-n", name, "-c", dir)
}

// Configure pins the window's name and tags it with its feature.
func Configure(r run.Runner, windowID, feature string) error {
	for _, opt := range [][]string{
		{"automatic-rename", "off"},
		{"allow-rename", "off"},
		{FeatureOption, feature},
	} {
		if _, err := r.Run("", "tmux", "set-option", "-w", "-t", windowID, opt[0], opt[1]); err != nil {
			return err
		}
	}
	return nil
}

// SendCommand types cmd into the window and presses Enter. The command is
// sent literally (-l) so tmux does not interpret key names.
func SendCommand(r run.Runner, windowID, cmd string) error {
	if _, err := r.Run("", "tmux", "send-keys", "-t", windowID, "-l", cmd); err != nil {
		return err
	}
	_, err := r.Run("", "tmux", "send-keys", "-t", windowID, "Enter")
	return err
}

// SelectWindow switches the client to the window.
func SelectWindow(r run.Runner, windowID string) error {
	_, err := r.Run("", "tmux", "select-window", "-t", windowID)
	return err
}

// KillWindow kills the window by ID.
func KillWindow(r run.Runner, windowID string) error {
	_, err := r.Run("", "tmux", "kill-window", "-t", windowID)
	return err
}

// ListWindows returns the windows of the current session.
func ListWindows(r run.Runner) ([]Window, error) {
	out, err := r.Run("", "tmux", "list-windows", "-F",
		"#{window_id}\t#{window_name}\t#{"+FeatureOption+"}")
	if err != nil {
		return nil, err
	}
	var windows []Window
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		w := Window{ID: parts[0]}
		if len(parts) > 1 {
			w.Name = parts[1]
		}
		if len(parts) > 2 {
			w.Feature = parts[2]
		}
		windows = append(windows, w)
	}
	return windows, nil
}

// FindWindow locates the window for a feature: first by the
// @jumux-feature option, then by exact window name.
func FindWindow(windows []Window, feature, windowName string) (Window, bool) {
	for _, w := range windows {
		if w.Feature == feature {
			return w, true
		}
	}
	for _, w := range windows {
		if w.Name == windowName {
			return w, true
		}
	}
	return Window{}, false
}

// CurrentWindowFeature returns the @jumux-feature option of the client's
// current window ("" if unset).
func CurrentWindowFeature(r run.Runner) (string, error) {
	return r.Run("", "tmux", "display-message", "-p", "#{"+FeatureOption+"}")
}
