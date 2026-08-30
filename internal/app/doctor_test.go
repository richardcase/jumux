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
		"Notification":[
			{"matcher":"permission_prompt","hooks":[{"type":"command","command":"jumux hook blocked"}]},
			{"matcher":"idle_prompt","hooks":[{"type":"command","command":"jumux hook waiting"}]}
		],
		"PostToolUseFailure":[{"hooks":[{"type":"command","command":"jumux hook error"}]}],
		"Stop":[{"hooks":[{"type":"command","command":"jumux hook done"}]}]
	}}`
	if err := os.WriteFile(settingsPath, []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	codexSettingsPath := filepath.Join(t.TempDir(), "hooks.json")
	codexSettingsJSON := `{"hooks":{
		"UserPromptSubmit":[{"hooks":[{"type":"command","command":"jumux hook working"}]}],
		"PostToolUse":[{"hooks":[{"type":"command","command":"jumux hook working"}]}],
		"PermissionRequest":[{"hooks":[{"type":"command","command":"jumux hook blocked"}]}],
		"Stop":[{"hooks":[{"type":"command","command":"jumux hook done"}]}]
	}}`
	if err := os.WriteFile(codexSettingsPath, []byte(codexSettingsJSON), 0o644); err != nil {
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
		case strings.HasPrefix(cmd, "gh --version"):
			return "gh version 2.50.0", nil
		case strings.HasPrefix(cmd, "glab --version"):
			return "glab 1.40.0", nil
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
		CodexSettings:  codexSettingsPath,
	}
	if err := a.Doctor(); err != nil {
		t.Fatalf("got %v, want nil.\noutput:\n%s", err, out.String())
	}
	got := out.String()
	if strings.Contains(got, "FAIL") {
		t.Errorf("expected no failures, got:\n%s", got)
	}
	for _, want := range []string{"[PASS] gh installed", "[PASS] glab installed"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q, got:\n%s", want, got)
		}
	}
}

func TestDoctorReportsFailures(t *testing.T) {
	mainRoot := doctorFixture(t, false) // not colocated: no .git
	settingsPath := filepath.Join(t.TempDir(), "missing-settings.json")
	codexSettingsPath := filepath.Join(t.TempDir(), "missing-hooks.json")

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
		case strings.HasPrefix(cmd, "gh --version"):
			return "", errors.New(`exec: "gh": executable file not found in $PATH`)
		case strings.HasPrefix(cmd, "glab --version"):
			return "", errors.New(`exec: "glab": executable file not found in $PATH`)
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
		CodexSettings:  codexSettingsPath,
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
		"[FAIL] Claude Code hooks installed — missing: UserPromptSubmit, PostToolUse, Notification (permission_prompt), Notification (idle_prompt), PostToolUseFailure, Stop",
		"[FAIL] Codex hooks installed — missing: UserPromptSubmit, PostToolUse, PermissionRequest, Stop",
		"[WARN] gh installed",
		"[WARN] glab installed",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "[FAIL] gh installed") || strings.Contains(got, "[FAIL] glab installed") {
		t.Errorf("gh/glab checks must be advisory (WARN), not FAIL, got:\n%s", got)
	}
}

// TestDoctorGHGlabAdvisory verifies that missing gh and glab are reported as
// warnings and do not, by themselves, fail doctor overall.
func TestDoctorGHGlabAdvisory(t *testing.T) {
	mainRoot := doctorFixture(t, true)
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	settingsJSON := `{"hooks":{
		"UserPromptSubmit":[{"hooks":[{"type":"command","command":"jumux hook working"}]}],
		"PostToolUse":[{"hooks":[{"type":"command","command":"jumux hook working"}]}],
		"Notification":[
			{"matcher":"permission_prompt","hooks":[{"type":"command","command":"jumux hook blocked"}]},
			{"matcher":"idle_prompt","hooks":[{"type":"command","command":"jumux hook waiting"}]}
		],
		"PostToolUseFailure":[{"hooks":[{"type":"command","command":"jumux hook error"}]}],
		"Stop":[{"hooks":[{"type":"command","command":"jumux hook done"}]}]
	}}`
	if err := os.WriteFile(settingsPath, []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	codexSettingsPath := filepath.Join(t.TempDir(), "hooks.json")
	codexSettingsJSON := `{"hooks":{
		"UserPromptSubmit":[{"hooks":[{"type":"command","command":"jumux hook working"}]}],
		"PostToolUse":[{"hooks":[{"type":"command","command":"jumux hook working"}]}],
		"PermissionRequest":[{"hooks":[{"type":"command","command":"jumux hook blocked"}]}],
		"Stop":[{"hooks":[{"type":"command","command":"jumux hook done"}]}]
	}}`
	if err := os.WriteFile(codexSettingsPath, []byte(codexSettingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		ghErr     error
		glabErr   error
		wantLines []string
	}{
		{
			name:      "both found",
			wantLines: []string{"[PASS] gh installed", "[PASS] glab installed"},
		},
		{
			name:      "gh not found",
			ghErr:     errors.New(`exec: "gh": executable file not found in $PATH`),
			wantLines: []string{"[WARN] gh installed", "[PASS] glab installed"},
		},
		{
			name:      "glab not found",
			glabErr:   errors.New(`exec: "glab": executable file not found in $PATH`),
			wantLines: []string{"[PASS] gh installed", "[WARN] glab installed"},
		},
		{
			name:      "both not found",
			ghErr:     errors.New(`exec: "gh": executable file not found in $PATH`),
			glabErr:   errors.New(`exec: "glab": executable file not found in $PATH`),
			wantLines: []string{"[WARN] gh installed", "[WARN] glab installed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				case strings.HasPrefix(cmd, "gh --version"):
					if tt.ghErr != nil {
						return "", tt.ghErr
					}
					return "gh version 2.50.0", nil
				case strings.HasPrefix(cmd, "glab --version"):
					if tt.glabErr != nil {
						return "", tt.glabErr
					}
					return "glab 1.40.0", nil
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
				CodexSettings:  codexSettingsPath,
			}
			if err := a.Doctor(); err != nil {
				t.Fatalf("got %v, want nil: doctor must still pass when gh/glab are missing.\noutput:\n%s", err, out.String())
			}
			got := out.String()
			for _, want := range tt.wantLines {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q, got:\n%s", want, got)
				}
			}
			if strings.Contains(got, "FAIL") {
				t.Errorf("expected no FAIL lines (gh/glab are advisory), got:\n%s", got)
			}
		})
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
