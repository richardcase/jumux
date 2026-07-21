package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardcase/agentmux/internal/run"
)

// fixture builds a fake main repo (myrepo with .jj/repo dir + .git) and an
// App whose runner answers the common jj/tmux queries. Tests tweak the
// handler map or App fields as needed.
type fixture struct {
	app      *App
	runner   *run.FakeRunner
	mainRoot string
	out, err *bytes.Buffer
	// scripted responses, keyed by command prefix (first match wins)
	responses map[string]string
	failOn    string // command prefix that returns an error
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	tmp := t.TempDir()
	mainRoot := filepath.Join(tmp, "myrepo")
	for _, d := range []string{filepath.Join(mainRoot, ".jj", "repo"), filepath.Join(mainRoot, ".git")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f := &fixture{
		mainRoot: mainRoot,
		out:      &bytes.Buffer{},
		err:      &bytes.Buffer{},
		responses: map[string]string{
			"jj root":              mainRoot,
			"jj workspace list":    "default: qq 11 (empty)\nauth: kk 22 stuff",
			"jj log":               "clean",
			"tmux new-window":      "@7",
			"tmux list-windows":    "@1\tzsh\t\n@2\tauth\tauth",
			"tmux display-message": "",
		},
	}
	f.runner = &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		if f.failOn != "" && strings.HasPrefix(cmd, f.failOn) {
			return "", errors.New("scripted failure: " + f.failOn)
		}
		for prefix, resp := range f.responses {
			if strings.HasPrefix(cmd, prefix) {
				return resp, nil
			}
		}
		return "", nil
	}}
	f.app = &App{
		Runner: f.runner,
		Out:    f.out,
		Errw:   f.err,
		In:     strings.NewReader(""),
		Getwd:  func() (string, error) { return mainRoot, nil },
		Getenv: func(k string) string {
			if k == "TMUX" {
				return "/tmp/tmux-1/default,123,0"
			}
			return ""
		},
	}
	return f
}

func (f *fixture) wsPath(feature string) string {
	return filepath.Join(filepath.Dir(f.mainRoot), "myrepo-"+feature)
}

func (f *fixture) assertRan(t *testing.T, substrings ...string) {
	t.Helper()
	lines := f.runner.CommandLines()
	for _, s := range substrings {
		if !strings.Contains(lines, s) {
			t.Errorf("expected a command containing %q; ran:\n%s", s, lines)
		}
	}
}

func (f *fixture) assertNotRan(t *testing.T, substrings ...string) {
	t.Helper()
	lines := f.runner.CommandLines()
	for _, s := range substrings {
		if strings.Contains(lines, s) {
			t.Errorf("expected no command containing %q; ran:\n%s", s, lines)
		}
	}
}

func TestAddHappyPath(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Add("billing"); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t,
		"jj workspace add --name billing -r trunk() "+f.wsPath("billing"),
		"tmux new-window -d -P -F #{window_id} -n billing -c "+f.wsPath("billing"),
		"tmux set-option -w -t @7 automatic-rename off",
		"tmux set-option -w -t @7 @agentmux-feature billing",
		"tmux send-keys -t @7 -l claude",
		"tmux send-keys -t @7 Enter",
		"tmux select-window -t @7",
	)
}

func TestAddUsesRepoConfigWithFeaturePlaceholder(t *testing.T) {
	f := newFixture(t)
	cfg := "agent = \"claude 'work on {feature}'\"\nselect_window = false\nwindow_prefix = \"ai-\"\nbase_revision = \"main\"\n"
	if err := os.WriteFile(filepath.Join(f.mainRoot, ".agentmux.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Add("billing"); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t,
		"jj workspace add --name billing -r main",
		"-n ai-billing",
		"tmux send-keys -t @7 -l claude 'work on billing'",
	)
	f.assertNotRan(t, "select-window")
}

func TestAddRejectsExistingWorkspace(t *testing.T) {
	f := newFixture(t)
	err := f.app.Add("auth")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("got %v", err)
	}
	f.assertNotRan(t, "workspace add", "new-window")
}

func TestAddRejectsInvalidNameAndOutsideTmux(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Add("bad name!"); err == nil {
		t.Error("expected invalid-name error")
	}
	f.app.Getenv = func(string) string { return "" }
	if err := f.app.Add("ok"); err == nil || !strings.Contains(err.Error(), "tmux") {
		t.Errorf("expected tmux-required error, got %v", err)
	}
}

func TestAddRollsBackWorkspaceWhenWindowFails(t *testing.T) {
	f := newFixture(t)
	f.failOn = "tmux new-window"
	if err := f.app.Add("billing"); err == nil {
		t.Fatal("expected error")
	}
	f.assertRan(t, "jj workspace add --name billing", "jj workspace forget billing")
}

func TestAddRollsBackEverythingWhenSendKeysFails(t *testing.T) {
	f := newFixture(t)
	f.failOn = "tmux send-keys"
	if err := f.app.Add("billing"); err == nil {
		t.Fatal("expected error")
	}
	f.assertRan(t, "tmux kill-window -t @7", "jj workspace forget billing")
}

func TestRemoveByName(t *testing.T) {
	f := newFixture(t)
	ws := f.wsPath("auth")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Remove("auth", false); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "jj workspace forget auth", "tmux kill-window -t @2")
	if _, err := os.Stat(ws); !os.IsNotExist(err) {
		t.Error("workspace dir should be deleted")
	}
}

func TestRemoveDirtyDeclinedAndForced(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = "dirty"
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.app.In = strings.NewReader("n\n")
	if err := f.app.Remove("auth", false); err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected abort, got %v", err)
	}
	f.assertNotRan(t, "workspace forget")

	// --force skips the check entirely.
	f2 := newFixture(t)
	f2.responses["jj log"] = "dirty"
	if err := os.MkdirAll(f2.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f2.app.Remove("auth", true); err != nil {
		t.Fatal(err)
	}
	f2.assertRan(t, "jj workspace forget auth")
	f2.assertNotRan(t, "jj log")
}

func TestRemoveRefusesDefault(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Remove("default", false); err == nil || !strings.Contains(err.Error(), "default") {
		t.Fatalf("got %v", err)
	}
}

func TestRemoveNothingFound(t *testing.T) {
	f := newFixture(t)
	f.responses["tmux list-windows"] = "@1\tzsh\t"
	if err := f.app.Remove("ghost", false); err == nil || !strings.Contains(err.Error(), "nothing to remove") {
		t.Fatalf("got %v", err)
	}
}

func TestRemoveStaleWorkspaceSkipsDirtyCheck(t *testing.T) {
	f := newFixture(t)
	// Listed in jj and has a window, but the directory is gone.
	if err := f.app.Remove("auth", false); err != nil {
		t.Fatal(err)
	}
	f.assertNotRan(t, "jj log")
	f.assertRan(t, "jj workspace forget auth", "tmux kill-window -t @2")
}

func TestRemoveInfersFeatureFromCwd(t *testing.T) {
	f := newFixture(t)
	ws := f.wsPath("auth")
	if err := os.MkdirAll(filepath.Join(ws, ".jj"), 0o755); err != nil {
		t.Fatal(err)
	}
	pointer := filepath.Join(f.mainRoot, ".jj", "repo")
	if err := os.WriteFile(filepath.Join(ws, ".jj", "repo"), []byte(pointer), 0o644); err != nil {
		t.Fatal(err)
	}
	f.app.Getwd = func() (string, error) { return ws, nil }
	f.responses["jj root"] = ws
	if err := f.app.Remove("", false); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "jj workspace forget auth")
}

func TestRemoveInfersFeatureFromWindowTag(t *testing.T) {
	f := newFixture(t)
	f.responses["tmux display-message"] = "auth"
	if err := f.app.Remove("", false); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "jj workspace forget auth")
}

func TestRemoveCannotInfer(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Remove("", false); err == nil || !strings.Contains(err.Error(), "cannot infer") {
		t.Fatalf("got %v", err)
	}
}

func TestListJoinsWorkspacesAndWindows(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.responses["jj workspace list"] = "default: qq 11\nauth: kk 22\nghost: gg 33"
	if err := f.app.List(); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "auth") || !strings.Contains(out, "@2") {
		t.Errorf("auth row missing window: %s", out)
	}
	if !strings.Contains(out, "stale") {
		t.Errorf("ghost row should be stale: %s", out)
	}
	if strings.Contains(out, "default") {
		t.Errorf("default workspace must be hidden: %s", out)
	}
}
