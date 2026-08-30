package app

import (
	"encoding/json"
	"fmt"
	"os"
)

// codexHookEvents maps Codex hook events to the agent status jumux records.
// Codex has no equivalent of Claude Code's idle notification or tool-use
// failure event, so "waiting" and "error" are not wired up here.
var codexHookEvents = []hookEvent{
	{"UserPromptSubmit", "", "working"},
	{"PostToolUse", "", "working"},
	{"PermissionRequest", "", "blocked"},
	{"Stop", "", "done"},
}

// ensureCodexHooks checks the Codex hooks file for the jumux status hooks
// and, when any are missing, offers to add them. It only ever reports
// problems on Errw — hook setup must never block feature creation.
func (a *App) ensureCodexHooks() {
	if a.CodexSettings == "" {
		return
	}
	hooks := map[string]json.RawMessage{}
	data, err := os.ReadFile(a.CodexSettings)
	switch {
	case err == nil:
		wrapper := struct {
			Hooks map[string]json.RawMessage `json:"hooks"`
		}{}
		if err := json.Unmarshal(data, &wrapper); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "warning: cannot parse %s: %v; skipping hook setup\n", a.CodexSettings, err)
			return
		}
		if wrapper.Hooks != nil {
			hooks = wrapper.Hooks
		}
	case !os.IsNotExist(err):
		_, _ = fmt.Fprintf(a.Errw, "warning: cannot read %s: %v; skipping hook setup\n", a.CodexSettings, err)
		return
	}

	missing := missingHookEventsFrom(hooks, codexHookEvents)
	if len(missing) == 0 {
		return
	}
	if !a.confirm(fmt.Sprintf("Codex hooks for agent status are not configured. Add them to %s?", a.CodexSettings)) {
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

	out, err := json.MarshalIndent(struct {
		Hooks map[string]json.RawMessage `json:"hooks"`
	}{Hooks: hooks}, "", "  ")
	if err != nil {
		return
	}
	out = append(out, '\n')
	if err := writeFileAtomic(a.CodexSettings, out); err != nil {
		_, _ = fmt.Fprintf(a.Errw, "warning: cannot write %s: %v\n", a.CodexSettings, err)
		return
	}
	_, _ = fmt.Fprintf(a.Out, "added jumux status hooks to %s\n", a.CodexSettings)
}

// missingCodexHookEvents reads settingsPath and returns which jumux status
// hook events are missing, without writing anything. A missing hooks file
// counts as every event missing.
func missingCodexHookEvents(settingsPath string) ([]hookEvent, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return codexHookEvents, nil
		}
		return nil, fmt.Errorf("reading %s: %w", settingsPath, err)
	}
	wrapper := struct {
		Hooks map[string]json.RawMessage `json:"hooks"`
	}{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", settingsPath, err)
	}
	return missingHookEventsFrom(wrapper.Hooks, codexHookEvents), nil
}

// missingHookEventsFrom returns the events in table with no entry, scoped
// to the right matcher, whose command invokes `jumux hook`.
func missingHookEventsFrom(hooks map[string]json.RawMessage, table []hookEvent) []hookEvent {
	var missing []hookEvent
	for _, he := range table {
		if hasJumuxHook(hooks[he.Event], he.Matcher) {
			continue
		}
		missing = append(missing, he)
	}
	return missing
}
