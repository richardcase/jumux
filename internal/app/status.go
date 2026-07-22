package app

import (
	"path/filepath"
	"sort"

	"github.com/richardcase/agentmux/internal/jj"
	"github.com/richardcase/agentmux/internal/run"
	"github.com/richardcase/agentmux/internal/tmuxctl"
)

// FeatureStatus is one sidebar row: an agentmux-tagged tmux window joined
// with the jj state of the workspace behind it.
type FeatureStatus struct {
	Repo        string // base name of the row's jj main root
	Feature     string
	Status      string // "clean" | "dirty" | "unknown"
	SessionID   string
	SessionName string
	WindowID    string
	Activity    bool
}

// featureStatuses builds rows from every window (any session) carrying the
// @agentmux-feature option. The repo is resolved from the window's current
// path, so multiple repos across sessions are handled without shared state.
// Rows whose path cannot be resolved get status "unknown" rather than being
// dropped.
func featureStatuses(r run.Runner, windows []tmuxctl.GlobalWindow) []FeatureStatus {
	mainRoots := map[string]string{} // path -> main root ("" on failure)
	var rows []FeatureStatus
	for _, w := range windows {
		if w.Feature == "" {
			continue
		}
		row := FeatureStatus{
			Feature:     w.Feature,
			Status:      "unknown",
			SessionID:   w.SessionID,
			SessionName: w.SessionName,
			WindowID:    w.ID,
			Activity:    w.Activity,
		}
		mainRoot, seen := mainRoots[w.Path]
		if !seen {
			if wsRoot, err := jj.Root(r, w.Path); err == nil {
				mainRoot, _ = jj.MainRoot(wsRoot)
			}
			mainRoots[w.Path] = mainRoot
		}
		if mainRoot != "" {
			row.Repo = filepath.Base(mainRoot)
			if dirty, err := jj.IsDirty(r, w.Path, w.Feature); err == nil {
				if dirty {
					row.Status = "dirty"
				} else {
					row.Status = "clean"
				}
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SessionName != rows[j].SessionName {
			return rows[i].SessionName < rows[j].SessionName
		}
		return rows[i].WindowID < rows[j].WindowID
	})
	return rows
}
