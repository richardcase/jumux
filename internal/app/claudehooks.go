package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hookEvent maps one Claude Code hook event to the agent status it records.
type hookEvent struct {
	Event  string
	Status string
}

var hookEvents = []hookEvent{
	{"UserPromptSubmit", "working"},
	{"PostToolUse", "working"},
	{"Notification", "waiting"},
	{"Stop", "done"},
}

// claudeHookEntry mirrors the Claude Code settings hooks schema.
type claudeHookEntry struct {
	Hooks []claudeHookCommand `json:"hooks"`
}

type claudeHookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// ensureClaudeHooks checks the Claude Code settings for the jumux status
// hooks and, when any are missing, offers to add them. It only ever reports
// problems on Errw — hook setup must never block feature creation.
func (a *App) ensureClaudeHooks() {
	if a.ClaudeSettings == "" {
		return
	}
	settings := map[string]json.RawMessage{}
	data, err := os.ReadFile(a.ClaudeSettings)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &settings); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "warning: cannot parse %s: %v; skipping hook setup\n", a.ClaudeSettings, err)
			return
		}
	case !os.IsNotExist(err):
		_, _ = fmt.Fprintf(a.Errw, "warning: cannot read %s: %v; skipping hook setup\n", a.ClaudeSettings, err)
		return
	}

	hooks := map[string]json.RawMessage{}
	if raw, ok := settings["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "warning: cannot parse hooks in %s: %v; skipping hook setup\n", a.ClaudeSettings, err)
			return
		}
	}

	missing := missingHookEvents(hooks)
	if len(missing) == 0 {
		return
	}
	if !a.confirm(fmt.Sprintf("Claude Code hooks for agent status are not configured. Add them to %s?", a.ClaudeSettings)) {
		return
	}

	for _, he := range missing {
		var entries []json.RawMessage
		if raw, ok := hooks[he.Event]; ok {
			if err := json.Unmarshal(raw, &entries); err != nil {
				_, _ = fmt.Fprintf(a.Errw, "warning: cannot parse %s hooks; skipping hook setup\n", he.Event)
				return
			}
		}
		entry, err := json.Marshal(claudeHookEntry{Hooks: []claudeHookCommand{
			{Type: "command", Command: "jumux hook " + he.Status},
		}})
		if err != nil {
			return
		}
		entries = append(entries, entry)
		merged, err := json.Marshal(entries)
		if err != nil {
			return
		}
		hooks[he.Event] = merged
	}
	mergedHooks, err := json.Marshal(hooks)
	if err != nil {
		return
	}
	settings["hooks"] = mergedHooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return
	}
	out = append(out, '\n')
	if err := writeFileAtomic(a.ClaudeSettings, out); err != nil {
		_, _ = fmt.Fprintf(a.Errw, "warning: cannot write %s: %v\n", a.ClaudeSettings, err)
		return
	}
	_, _ = fmt.Fprintf(a.Out, "added jumux status hooks to %s\n", a.ClaudeSettings)
}

// missingHookEvents returns the hook events with no entry whose command
// invokes `jumux hook`.
func missingHookEvents(hooks map[string]json.RawMessage) []hookEvent {
	var missing []hookEvent
	for _, he := range hookEvents {
		if raw, ok := hooks[he.Event]; ok && strings.Contains(string(raw), "jumux hook") {
			continue
		}
		missing = append(missing, he)
	}
	return missing
}

// writeFileAtomic writes data via a temp file + rename, creating parent dirs.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
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
	return os.Rename(tmp.Name(), path)
}
