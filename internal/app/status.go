package app

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/run"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

// FeatureStatus is one sidebar row: an jumux-tagged tmux window joined
// with the jj state of the workspace behind it.
type FeatureStatus struct {
	Repo        string // base name of the row's jj main root
	Feature     string
	Status      string // "clean" | "dirty" | "unknown"
	AgentStatus string // "working" | "waiting" | "done" | "blocked" | "error" | "" (unknown)
	SessionID   string
	SessionName string
	WindowID    string
	Activity    bool
	Stale       bool // idle beyond the configured stale threshold
}

// featureStatuses builds rows from every window (any session) carrying the
// @jumux-feature option. The repo is resolved from the window's current
// path, so multiple repos across sessions are handled without shared state.
// Rows whose path cannot be resolved get status "unknown" rather than being
// dropped.
// agent maps window IDs to hook-reported agent statuses (nil when no agent
// state is available). hookUpdates maps window IDs to their last hook
// update time (nil to skip hook-based activity). now and staleAfter drive
// staleness: a row is Stale when its last jj change and last hook update
// are both older than staleAfter (or unknown); staleAfter <= 0 disables it.
func featureStatuses(r run.Runner, windows []tmuxctl.GlobalWindow, agent map[string]agentstate.Status, hookUpdates map[string]time.Time, now time.Time, staleAfter time.Duration) []FeatureStatus {
	mainRoots := map[string]string{} // path -> main root ("" on failure)
	var rows []FeatureStatus
	for _, w := range windows {
		if w.Feature == "" {
			continue
		}
		row := FeatureStatus{
			Feature:     w.Feature,
			Status:      "unknown",
			AgentStatus: string(agent[w.ID]),
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
		if staleAfter > 0 {
			var latest time.Time
			found := false
			if t, err := jj.LastChangeTime(r, w.Path, w.Feature); err == nil {
				latest = t
				found = true
			}
			if t, ok := hookUpdates[w.ID]; ok && (!found || t.After(latest)) {
				latest = t
				found = true
			}
			row.Stale = found && now.Sub(latest) > staleAfter
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SessionName != rows[j].SessionName {
			return rows[i].SessionName < rows[j].SessionName
		}
		return rows[i].WindowID < rows[j].WindowID
	})
	// When more than one repo is present, group rows by repo (stable, so
	// each repo's rows keep their session/window order) so the sidebar can
	// render a header per repo instead of an interleaved list.
	if multipleRepos(rows) {
		sort.SliceStable(rows, func(i, j int) bool {
			return rows[i].Repo < rows[j].Repo
		})
	}
	return rows
}

// multipleRepos reports whether rows resolve to two or more distinct repos.
func multipleRepos(rows []FeatureStatus) bool {
	repos := map[string]bool{}
	for _, row := range rows {
		if row.Repo != "" {
			repos[row.Repo] = true
		}
	}
	return len(repos) > 1
}
