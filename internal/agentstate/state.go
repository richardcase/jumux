// Package agentstate stores per-window agent liveness status as small JSON
// files. It knows nothing about jj or tmux: callers pass window IDs and a
// state directory, so the package is testable with a temp dir.
package agentstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Status is the agent liveness reported by hooks.
type Status string

const (
	Working Status = "working"
	Waiting Status = "waiting"
	Done    Status = "done"
	Blocked Status = "blocked"
	Error   Status = "error"
)

// Valid reports whether s is one of the known statuses.
func Valid(s Status) bool {
	switch s {
	case Working, Waiting, Done, Blocked, Error:
		return true
	}
	return false
}

// Entry is one window's recorded agent status.
type Entry struct {
	WindowID  string    `json:"window_id"`
	PaneID    string    `json:"pane_id"`
	Status    Status    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WorkingTTL is how long a "working" entry stays trusted without an update;
// past it the agent is assumed dead and the status reads as unknown.
const WorkingTTL = 15 * time.Minute

// Dir resolves the state directory: $XDG_STATE_HOME/jumux/status, falling
// back to ~/.local/state/jumux/status.
func Dir(getenv func(string) string) string {
	base := getenv("XDG_STATE_HOME")
	if base == "" {
		base = filepath.Join(getenv("HOME"), ".local", "state")
	}
	return filepath.Join(base, "jumux", "status")
}

// fileFor maps a tmux window ID (e.g. "@5") to a state file name ("w5.json").
func fileFor(windowID string) string {
	var b strings.Builder
	b.WriteByte('w')
	for _, r := range windowID {
		if r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String() + ".json"
}

// Write records an entry atomically (temp file + rename), creating the
// directory if needed.
func Write(dir string, e Entry) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, fileFor(e.WindowID)))
}

// ReadAll returns windowID -> status for every readable entry. Unparseable
// files are skipped, a missing directory yields an empty map, and "working"
// entries older than WorkingTTL are dropped (dead agents shouldn't spin
// forever; waiting/done/blocked/error are stable end states with no TTL).
func ReadAll(dir string, now time.Time) map[string]Status {
	statuses := map[string]Status{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return statuses
	}
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, de.Name()))
		if err != nil {
			continue
		}
		var e Entry
		if err := json.Unmarshal(data, &e); err != nil || e.WindowID == "" || !Valid(e.Status) {
			continue
		}
		if e.Status == Working && now.Sub(e.UpdatedAt) > WorkingTTL {
			continue
		}
		statuses[e.WindowID] = e.Status
	}
	return statuses
}

// LastUpdated returns windowID -> UpdatedAt for every readable entry,
// regardless of status validity or the "working" TTL — callers use it
// purely as an activity timestamp (e.g. staleness detection), not a status.
func LastUpdated(dir string) map[string]time.Time {
	updated := map[string]time.Time{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return updated
	}
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, de.Name()))
		if err != nil {
			continue
		}
		var e Entry
		if err := json.Unmarshal(data, &e); err != nil || e.WindowID == "" {
			continue
		}
		updated[e.WindowID] = e.UpdatedAt
	}
	return updated
}

// Remove deletes the entry for a window; a missing entry is not an error.
func Remove(dir, windowID string) error {
	err := os.Remove(filepath.Join(dir, fileFor(windowID)))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Prune deletes entries for windows not in the live set.
func Prune(dir string, live map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var firstErr error
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, de.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var e Entry
		if err := json.Unmarshal(data, &e); err == nil && live[e.WindowID] {
			continue
		}
		if err := os.Remove(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
