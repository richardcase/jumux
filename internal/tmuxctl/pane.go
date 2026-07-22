package tmuxctl

import (
	"strconv"
	"strings"

	"github.com/richardcase/agentmux/internal/run"
)

// SidebarOption is the tmux pane option that marks agentmux sidebar panes.
const SidebarOption = "@agentmux-sidebar"

// Pane is one row from list-panes.
type Pane struct {
	ID       string // %N
	WindowID string // @N
	Sidebar  bool   // @agentmux-sidebar option set
}

// GlobalWindow is one row from list-windows -a (any session).
type GlobalWindow struct {
	SessionID   string // $N
	SessionName string
	ID          string // @N
	Name        string
	Feature     string // value of @agentmux-feature, "" if unset
	Path        string // pane_current_path of the active pane
	Activity    bool   // window_activity_flag
}

// ListAllPanes returns every pane across all sessions.
func ListAllPanes(r run.Runner) ([]Pane, error) {
	out, err := r.Run("", "tmux", "list-panes", "-a", "-F",
		"#{pane_id}\t#{window_id}\t#{"+SidebarOption+"}")
	if err != nil {
		return nil, err
	}
	var panes []Pane
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		p := Pane{ID: parts[0]}
		if len(parts) > 1 {
			p.WindowID = parts[1]
		}
		if len(parts) > 2 {
			p.Sidebar = parts[2] == "1"
		}
		panes = append(panes, p)
	}
	return panes, nil
}

// ListAllWindows returns every window across all sessions.
func ListAllWindows(r run.Runner) ([]GlobalWindow, error) {
	out, err := r.Run("", "tmux", "list-windows", "-a", "-F",
		"#{session_id}\t#{session_name}\t#{window_id}\t#{window_name}\t#{"+
			FeatureOption+"}\t#{pane_current_path}\t#{window_activity_flag}")
	if err != nil {
		return nil, err
	}
	var windows []GlobalWindow
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		var w GlobalWindow
		fields := []*string{&w.SessionID, &w.SessionName, &w.ID, &w.Name, &w.Feature, &w.Path}
		for i, f := range fields {
			if i < len(parts) {
				*f = parts[i]
			}
		}
		if len(parts) > 6 {
			w.Activity = parts[6] == "1"
		}
		windows = append(windows, w)
	}
	return windows, nil
}

// SplitWindowLeft creates a pane on the left edge of the window running cmd,
// without stealing focus, and returns the new pane's ID.
func SplitWindowLeft(r run.Runner, windowID, dir string, width int, cmd ...string) (string, error) {
	args := []string{"split-window", "-h", "-b", "-d", "-l", strconv.Itoa(width),
		"-t", windowID, "-c", dir, "-P", "-F", "#{pane_id}"}
	args = append(args, cmd...)
	return r.Run("", "tmux", args...)
}

// SetPaneOption sets a pane-scoped option.
func SetPaneOption(r run.Runner, paneID, opt, val string) error {
	_, err := r.Run("", "tmux", "set-option", "-p", "-t", paneID, opt, val)
	return err
}

// KillPane kills the pane by ID.
func KillPane(r run.Runner, paneID string) error {
	_, err := r.Run("", "tmux", "kill-pane", "-t", paneID)
	return err
}

// SwitchToWindow makes windowID current in its session and switches the
// client to that session, so jumps work across sessions.
func SwitchToWindow(r run.Runner, sessionID, windowID string) error {
	if _, err := r.Run("", "tmux", "select-window", "-t", windowID); err != nil {
		return err
	}
	_, err := r.Run("", "tmux", "switch-client", "-t", sessionID)
	return err
}

// PaneWindowActive reports whether the pane's window is its session's
// current window (i.e. the pane is potentially visible).
func PaneWindowActive(r run.Runner, paneID string) (bool, error) {
	out, err := r.Run("", "tmux", "display-message", "-p", "-t", paneID, "#{window_active}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "1", nil
}
