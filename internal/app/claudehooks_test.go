package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hooksApp(t *testing.T, settingsPath, answer string) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	errw := &bytes.Buffer{}
	return &App{
		Out:            out,
		Errw:           errw,
		In:             strings.NewReader(answer),
		ClaudeSettings: settingsPath,
	}, out, errw
}

func readSettings(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings not valid JSON: %v\n%s", err, data)
	}
	return settings
}

func hookEventsIn(t *testing.T, settings map[string]json.RawMessage) map[string]string {
	t.Helper()
	got := map[string]string{}
	if raw, ok := settings["hooks"]; ok {
		var hooks map[string]json.RawMessage
		if err := json.Unmarshal(raw, &hooks); err != nil {
			t.Fatal(err)
		}
		for event, entries := range hooks {
			got[event] = string(entries)
		}
	}
	return got
}

func TestEnsureClaudeHooksCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	a, out, _ := hooksApp(t, path, "y\n")
	a.ensureClaudeHooks()

	events := hookEventsIn(t, readSettings(t, path))
	for event, status := range map[string]string{
		"UserPromptSubmit": "working",
		"PostToolUse":      "working",
		"Notification":     "waiting",
		"Stop":             "done",
	} {
		if !strings.Contains(events[event], "jumux hook "+status) {
			t.Errorf("event %s missing hook %q: %s", event, status, events[event])
		}
	}
	if !strings.Contains(out.String(), "added jumux status hooks") {
		t.Errorf("expected confirmation output, got %q", out.String())
	}
}

func TestEnsureClaudeHooksMergesPartial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	existing := `{
  "model": "opus",
  "hooks": {
    "Stop": [{"hooks": [{"type": "command", "command": "jumux hook done"}]}],
    "Notification": [{"hooks": [{"type": "command", "command": "say ding"}]}]
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _, _ := hooksApp(t, path, "yes\n")
	a.ensureClaudeHooks()

	settings := readSettings(t, path)
	if string(settings["model"]) != `"opus"` {
		t.Errorf("unrelated settings should survive, got model=%s", settings["model"])
	}
	events := hookEventsIn(t, settings)
	if !strings.Contains(events["UserPromptSubmit"], "jumux hook working") {
		t.Errorf("missing UserPromptSubmit hook: %s", events["UserPromptSubmit"])
	}
	if !strings.Contains(events["Notification"], "say ding") ||
		!strings.Contains(events["Notification"], "jumux hook waiting") {
		t.Errorf("Notification should keep the existing entry and gain ours: %s", events["Notification"])
	}
	if strings.Count(events["Stop"], "jumux hook done") != 1 {
		t.Errorf("Stop hook should not be duplicated: %s", events["Stop"])
	}
}

func TestEnsureClaudeHooksAlreadyConfigured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	full := `{"hooks": {
  "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "jumux hook working"}]}],
  "PostToolUse": [{"hooks": [{"type": "command", "command": "jumux hook working"}]}],
  "Notification": [{"hooks": [{"type": "command", "command": "jumux hook waiting"}]}],
  "Stop": [{"hooks": [{"type": "command", "command": "jumux hook done"}]}]
}}`
	if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _, errw := hooksApp(t, path, "")
	a.ensureClaudeHooks()
	if strings.Contains(errw.String(), "[y/N]") {
		t.Errorf("should not prompt when hooks exist: %q", errw.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != full {
		t.Errorf("file should be untouched:\n%s", data)
	}
}

func TestEnsureClaudeHooksDeclined(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	a, _, errw := hooksApp(t, path, "n\n")
	a.ensureClaudeHooks()
	if !strings.Contains(errw.String(), "[y/N]") {
		t.Errorf("expected a prompt, got %q", errw.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("declining should not create the file")
	}
}

func TestEnsureClaudeHooksInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _, errw := hooksApp(t, path, "y\n")
	a.ensureClaudeHooks()
	if !strings.Contains(errw.String(), "cannot parse") {
		t.Errorf("expected a parse warning, got %q", errw.String())
	}
}

func TestEnsureClaudeHooksNoPathConfigured(t *testing.T) {
	a, _, errw := hooksApp(t, "", "y\n")
	a.ensureClaudeHooks()
	if errw.Len() != 0 {
		t.Errorf("empty ClaudeSettings should be a silent no-op, got %q", errw.String())
	}
}
