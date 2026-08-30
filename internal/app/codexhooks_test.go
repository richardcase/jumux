package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func codexHooksApp(t *testing.T, settingsPath, answer string) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	errw := &bytes.Buffer{}
	return &App{
		Out:           out,
		Errw:          errw,
		In:            strings.NewReader(answer),
		CodexSettings: settingsPath,
	}, out, errw
}

func readCodexHooks(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := struct {
		Hooks map[string]json.RawMessage `json:"hooks"`
	}{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("hooks file not valid JSON: %v\n%s", err, data)
	}
	return wrapper.Hooks
}

func TestEnsureCodexHooksCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".codex", "hooks.json")
	a, out, _ := codexHooksApp(t, path, "y\n")
	a.ensureCodexHooks()

	events := readCodexHooks(t, path)
	for event, status := range map[string]string{
		"UserPromptSubmit":  "working",
		"PostToolUse":       "working",
		"PermissionRequest": "blocked",
		"Stop":              "done",
	} {
		if !strings.Contains(string(events[event]), "jumux hook "+status) {
			t.Errorf("event %s missing hook %q: %s", event, status, events[event])
		}
	}
	if !strings.Contains(out.String(), "added jumux status hooks") {
		t.Errorf("expected confirmation output, got %q", out.String())
	}
}

func TestEnsureCodexHooksMergesPartial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	existing := `{
  "hooks": {
    "Stop": [{"hooks": [{"type": "command", "command": "jumux hook done"}]}],
    "PreToolUse": [{"hooks": [{"type": "command", "command": "some-other-hook"}]}]
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _, _ := codexHooksApp(t, path, "yes\n")
	a.ensureCodexHooks()

	events := readCodexHooks(t, path)
	if !strings.Contains(string(events["UserPromptSubmit"]), "jumux hook working") {
		t.Errorf("missing UserPromptSubmit hook: %s", events["UserPromptSubmit"])
	}
	if !strings.Contains(string(events["PreToolUse"]), "some-other-hook") {
		t.Errorf("unrelated PreToolUse hook should survive: %s", events["PreToolUse"])
	}
	if strings.Count(string(events["Stop"]), "jumux hook done") != 1 {
		t.Errorf("Stop hook should not be duplicated: %s", events["Stop"])
	}
}

func TestEnsureCodexHooksAlreadyConfigured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	full := `{"hooks": {
  "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "jumux hook working"}]}],
  "PostToolUse": [{"hooks": [{"type": "command", "command": "jumux hook working"}]}],
  "PermissionRequest": [{"hooks": [{"type": "command", "command": "jumux hook blocked"}]}],
  "Stop": [{"hooks": [{"type": "command", "command": "jumux hook done"}]}]
}}`
	if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _, errw := codexHooksApp(t, path, "")
	a.ensureCodexHooks()
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

func TestEnsureCodexHooksDeclined(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	a, _, errw := codexHooksApp(t, path, "n\n")
	a.ensureCodexHooks()
	if !strings.Contains(errw.String(), "[y/N]") {
		t.Errorf("expected a prompt, got %q", errw.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("declining should not create the file")
	}
}

func TestEnsureCodexHooksInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _, errw := codexHooksApp(t, path, "y\n")
	a.ensureCodexHooks()
	if !strings.Contains(errw.String(), "cannot parse") {
		t.Errorf("expected a parse warning, got %q", errw.String())
	}
}

func TestEnsureCodexHooksNoPathConfigured(t *testing.T) {
	a, _, errw := codexHooksApp(t, "", "y\n")
	a.ensureCodexHooks()
	if errw.Len() != 0 {
		t.Errorf("empty CodexSettings should be a silent no-op, got %q", errw.String())
	}
}

func TestMissingCodexHookEventsNoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	missing, err := missingCodexHookEvents(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != len(codexHookEvents) {
		t.Errorf("expected all %d events missing, got %d", len(codexHookEvents), len(missing))
	}
}
