package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hookEvent maps one Claude Code hook event, optionally scoped to a
// Notification matcher, to the agent status it records.
type hookEvent struct {
	Event   string
	Matcher string // "" for events with no matcher (e.g. Stop)
	Status  string
}

var hookEvents = []hookEvent{
	{"UserPromptSubmit", "", "working"},
	{"PostToolUse", "", "working"},
	{"Notification", "permission_prompt", "blocked"},
	{"Notification", "idle_prompt", "waiting"},
	{"PostToolUseFailure", "", "error"},
	{"Stop", "", "done"},
}

// claudeHookEntry mirrors the Claude Code settings hooks schema.
type claudeHookEntry struct {
	Matcher string              `json:"matcher,omitempty"`
	Hooks   []claudeHookCommand `json:"hooks"`
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
		var entries []claudeHookEntry
		if raw, ok := hooks[he.Event]; ok {
			if err := json.Unmarshal(raw, &entries); err != nil {
				_, _ = fmt.Fprintf(a.Errw, "warning: cannot parse %s hooks; skipping hook setup\n", he.Event)
				return
			}
		}
		entries = removeSupersededEntries(entries, he)
		entries = append(entries, claudeHookEntry{
			Matcher: he.Matcher,
			Hooks: []claudeHookCommand{
				{Type: "command", Command: "jumux hook " + he.Status},
			},
		})
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

// missingClaudeHookEvents reads settingsPath and returns which jumux status
// hook events are missing, without writing anything. A missing settings
// file counts as every event missing.
func missingClaudeHookEvents(settingsPath string) ([]hookEvent, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return hookEvents, nil
		}
		return nil, fmt.Errorf("reading %s: %w", settingsPath, err)
	}
	settings := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", settingsPath, err)
	}
	hooks := map[string]json.RawMessage{}
	if raw, ok := settings["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return nil, fmt.Errorf("parsing hooks in %s: %w", settingsPath, err)
		}
	}
	return missingHookEvents(hooks), nil
}

// missingHookEvents returns the hook events with no entry, scoped to the
// right matcher, whose command invokes `jumux hook`.
func missingHookEvents(hooks map[string]json.RawMessage) []hookEvent {
	return missingHookEventsFrom(hooks, hookEvents)
}

// hasJumuxHook reports whether raw (an event's hook entry array) already
// contains a jumux-managed command scoped to matcher.
func hasJumuxHook(raw json.RawMessage, matcher string) bool {
	if raw == nil {
		return false
	}
	var entries []claudeHookEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return false
	}
	for _, e := range entries {
		if e.Matcher != matcher {
			continue
		}
		for _, c := range e.Hooks {
			if strings.Contains(c.Command, "jumux hook") {
				return true
			}
		}
	}
	return false
}

// removeSupersededEntries drops legacy jumux entries that a new
// matcher-scoped entry replaces: the old bare Notification hook (no
// matcher) reported "waiting" for every notification, now split into
// permission_prompt/idle_prompt-scoped entries.
func removeSupersededEntries(entries []claudeHookEntry, he hookEvent) []claudeHookEntry {
	if he.Event != "Notification" || he.Matcher == "" {
		return entries
	}
	var kept []claudeHookEntry
	for _, e := range entries {
		if e.Matcher == "" && hasCommand(e.Hooks, "jumux hook waiting") {
			continue
		}
		kept = append(kept, e)
	}
	return kept
}

// hasCommand reports whether hooks contains a command exactly matching cmd.
func hasCommand(hooks []claudeHookCommand, cmd string) bool {
	for _, c := range hooks {
		if c.Command == cmd {
			return true
		}
	}
	return false
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
