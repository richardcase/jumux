package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

// doctorFixture builds a main jj repo root, optionally colocated with git.
func doctorFixture(t *testing.T, colocated bool) (mainRoot string) {
	t.Helper()
	tmp := t.TempDir()
	mainRoot = filepath.Join(tmp, "myrepo")
	if err := os.MkdirAll(filepath.Join(mainRoot, ".jj", "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if colocated {
		if err := os.MkdirAll(filepath.Join(mainRoot, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return mainRoot
}

func tmuxEnv(inTmux bool) func(string) string {
	return func(k string) string {
		if inTmux && k == "TMUX" {
			return "/tmp/tmux-1/default,123,0"
		}
		return ""
	}
}

func TestDoctorAllPass(t *testing.T) {
	mainRoot := doctorFixture(t, true)
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	settingsJSON := `{"hooks":{
		"UserPromptSubmit":[{"hooks":[{"type":"command","command":"jumux hook working"}]}],
		"PostToolUse":[{"hooks":[{"type":"command","command":"jumux hook working"}]}],
		"Notification":[{"hooks":[{"type":"command","command":"jumux hook waiting"}]}],
		"Stop":[{"hooks":[{"type":"command","command":"jumux hook done"}]}]
	}}`
	if err := os.WriteFile(settingsPath, []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "jj root"):
			return dir, nil
		case strings.HasPrefix(cmd, "jj --version"):
			return "jj 0.20.0", nil
		case strings.HasPrefix(cmd, "jj log"):
			return "", nil
		case strings.HasPrefix(cmd, "tmux list-sessions"):
			return "main: 1 windows", nil
		}
		return "", nil
	}}
	out := &strings.Builder{}
	a := &App{
		Runner:         fr,
		Out:            out,
		Errw:           &strings.Builder{},
		Getwd:          func() (string, error) { return mainRoot, nil },
		Getenv:         tmuxEnv(true),
		ClaudeSettings: settingsPath,
	}
	if err := a.Doctor(); err != nil {
		t.Fatalf("got %v, want nil.\noutput:\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "FAIL") {
		t.Errorf("expected no failures, got:\n%s", out.String())
	}
}

func TestDoctorReportsFailures(t *testing.T) {
	mainRoot := doctorFixture(t, false) // not colocated: no .git
	settingsPath := filepath.Join(t.TempDir(), "missing-settings.json")

	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "jj root"):
			return dir, nil
		case strings.HasPrefix(cmd, "jj --version"):
			return "", errors.New(`exec: "jj": executable file not found in $PATH`)
		case strings.HasPrefix(cmd, "jj log"):
			return "", errors.New(`revset "trunk()" doesn't resolve to any revisions`)
		case strings.HasPrefix(cmd, "tmux list-sessions"):
			return "", errors.New("no server running")
		}
		return "", nil
	}}
	out := &strings.Builder{}
	a := &App{
		Runner:         fr,
		Out:            out,
		Errw:           &strings.Builder{},
		Getwd:          func() (string, error) { return mainRoot, nil },
		Getenv:         tmuxEnv(false),
		ClaudeSettings: settingsPath,
	}
	err := a.Doctor()
	if err == nil {
		t.Fatal("got nil error, want non-nil when checks fail")
	}
	got := out.String()
	for _, want := range []string{
		"[FAIL] jj installed",
		"[FAIL] repo colocated with git",
		"[FAIL] tmux running",
		"[FAIL] base_revision",
		"[FAIL] Claude Code hooks installed — missing: UserPromptSubmit, PostToolUse, Notification, Stop",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q, got:\n%s", want, got)
		}
	}
}

func TestDoctorDegradesWhenNotInRepo(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "jj root"):
			return "", errors.New(`There is no jj repo in "."`)
		case strings.HasPrefix(cmd, "jj --version"):
			return "jj 0.20.0", nil
		case strings.HasPrefix(cmd, "tmux list-sessions"):
			return "main: 1 windows", nil
		}
		return "", nil
	}}
	out := &strings.Builder{}
	a := &App{
		Runner:         fr,
		Out:            out,
		Errw:           &strings.Builder{},
		Getwd:          func() (string, error) { return "/nowhere", nil },
		Getenv:         tmuxEnv(true),
		ClaudeSettings: "",
	}
	err := a.Doctor()
	if err == nil {
		t.Fatal("want error when the colocation/base_revision checks fail")
	}
	got := out.String()
	if !strings.Contains(got, "[FAIL] repo colocated with git") {
		t.Errorf("expected colocated check to fail, got:\n%s", got)
	}
	if !strings.Contains(got, "[FAIL] base_revision resolves — skipped") {
		t.Errorf("expected base_revision check to report skipped, got:\n%s", got)
	}
	// Checks that don't depend on repo context still run independently.
	if !strings.Contains(got, "[PASS] jj installed") {
		t.Errorf("jj installed check should still run, got:\n%s", got)
	}
	if !strings.Contains(got, "[PASS] tmux running") {
		t.Errorf("tmux check should still run, got:\n%s", got)
	}
}
