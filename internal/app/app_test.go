package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/run"
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
	if err := f.app.Add("billing", "", ""); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t,
		"jj workspace add --name billing -r trunk() "+f.wsPath("billing"),
		"tmux new-window -d -P -F #{window_id} -n billing -c "+f.wsPath("billing"),
		"tmux set-option -w -t @7 automatic-rename off",
		"tmux set-option -w -t @7 @jumux-feature billing",
		"tmux send-keys -t @7 -l claude",
		"tmux send-keys -t @7 Enter",
		"tmux select-window -t @7",
	)
}

func TestAddUsesRepoConfigWithFeaturePlaceholder(t *testing.T) {
	f := newFixture(t)
	cfg := "agent = \"claude 'work on {feature}'\"\nselect_window = false\nwindow_prefix = \"ai-\"\nbase_revision = \"main\"\n"
	if err := os.WriteFile(filepath.Join(f.mainRoot, ".jumux.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Add("billing", "", ""); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t,
		"jj workspace add --name billing -r main",
		"-n ai-billing",
		"tmux send-keys -t @7 -l claude 'work on billing'",
	)
	f.assertNotRan(t, "select-window")
}

func TestAddAgentOverride(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Add("billing", "aider 'work on {feature}'", ""); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "tmux send-keys -t @7 -l aider 'work on billing'")
	f.assertNotRan(t, "-l claude")
}

func TestAddAgentOverrideBeatsRepoConfig(t *testing.T) {
	f := newFixture(t)
	cfg := "agent = \"claude 'work on {feature}'\"\n"
	if err := os.WriteFile(filepath.Join(f.mainRoot, ".jumux.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Add("billing", "aider", ""); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "tmux send-keys -t @7 -l aider")
	f.assertNotRan(t, "-l claude")
}

func TestAddUsesTemplate(t *testing.T) {
	f := newFixture(t)
	cfg := "[templates.bugfix]\nagent = \"claude 'fix {feature}'\"\nbase_revision = \"main\"\nwindow_prefix = \"bug-\"\n"
	if err := os.WriteFile(filepath.Join(f.mainRoot, ".jumux.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Add("billing", "", "bugfix"); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t,
		"jj workspace add --name billing -r main",
		"-n bug-billing",
		"tmux send-keys -t @7 -l claude 'fix billing'",
	)
}

func TestAddTemplateOverriddenByAgentFlag(t *testing.T) {
	f := newFixture(t)
	cfg := "[templates.bugfix]\nagent = \"claude 'fix {feature}'\"\n"
	if err := os.WriteFile(filepath.Join(f.mainRoot, ".jumux.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Add("billing", "aider", "bugfix"); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "tmux send-keys -t @7 -l aider")
	f.assertNotRan(t, "-l claude")
}

func TestAddUnknownTemplateErrors(t *testing.T) {
	f := newFixture(t)
	err := f.app.Add("billing", "", "nope")
	if err == nil || !strings.Contains(err.Error(), "unknown template") {
		t.Fatalf("got %v", err)
	}
	f.assertNotRan(t, "workspace add", "new-window")
}

func TestAddFallsBackToParentWhenTrunkFails(t *testing.T) {
	f := newFixture(t)
	f.failOn = "jj workspace add --name billing -r trunk()"
	if err := f.app.Add("billing", "", ""); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "jj workspace add --name billing -r @- "+f.wsPath("billing"))
	if !strings.Contains(f.err.String(), "warning:") {
		t.Errorf("expected a warning about the fallback, got: %s", f.err.String())
	}
}

func TestAddDoesNotFallBackWithExplicitBaseRevision(t *testing.T) {
	f := newFixture(t)
	cfg := "base_revision = \"main\"\n"
	if err := os.WriteFile(filepath.Join(f.mainRoot, ".jumux.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	f.failOn = "jj workspace add --name billing -r main"
	err := f.app.Add("billing", "", "")
	if err == nil || !strings.Contains(err.Error(), "scripted failure") {
		t.Fatalf("expected the original error, got %v", err)
	}
	f.assertNotRan(t, "-r @-")
}

func TestAddRejectsExistingWorkspace(t *testing.T) {
	f := newFixture(t)
	err := f.app.Add("auth", "", "")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("got %v", err)
	}
	f.assertNotRan(t, "workspace add", "new-window")
}

func TestAddRejectsInvalidNameAndOutsideTmux(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Add("bad name!", "", ""); err == nil {
		t.Error("expected invalid-name error")
	}
	f.app.Getenv = func(string) string { return "" }
	if err := f.app.Add("ok", "", ""); err == nil || !strings.Contains(err.Error(), "tmux") {
		t.Errorf("expected tmux-required error, got %v", err)
	}
}

func TestAddRollsBackWorkspaceWhenWindowFails(t *testing.T) {
	f := newFixture(t)
	f.failOn = "tmux new-window"
	if err := f.app.Add("billing", "", ""); err == nil {
		t.Fatal("expected error")
	}
	f.assertRan(t, "jj workspace add --name billing", "jj workspace forget billing")
}

func TestAddRollsBackEverythingWhenSendKeysFails(t *testing.T) {
	f := newFixture(t)
	f.failOn = "tmux send-keys"
	if err := f.app.Add("billing", "", ""); err == nil {
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

func TestRemoveAllDoneRemovesOnlyDoneFeatures(t *testing.T) {
	f := newFixture(t)
	f.app.StateDir = t.TempDir()
	f.responses["jj workspace list"] = "default: qq 11\nauth: kk 22\nbilling: bb 33"
	f.responses["tmux list-windows"] = "@1\tzsh\t\n@2\tauth\tauth\n@3\tbilling\tbilling"
	if err := agentstate.Write(f.app.StateDir, agentstate.Entry{WindowID: "@2", Status: agentstate.Done, UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := agentstate.Write(f.app.StateDir, agentstate.Entry{WindowID: "@3", Status: agentstate.Working, UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := f.app.RemoveAllDone(false); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "jj workspace forget auth", "tmux kill-window -t @2")
	f.assertNotRan(t, "jj workspace forget billing", "tmux kill-window -t @3")
}

func TestRemoveAllDoneNoneDone(t *testing.T) {
	f := newFixture(t)
	f.app.StateDir = t.TempDir()
	if err := f.app.RemoveAllDone(false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "no done features to remove") {
		t.Errorf("expected a no-op message, got: %s", f.out.String())
	}
	f.assertNotRan(t, "jj workspace forget")
}

func TestRemoveAllDoneRemovesCurrentFeatureLast(t *testing.T) {
	f := newFixture(t)
	f.app.StateDir = t.TempDir()
	f.responses["jj workspace list"] = "default: qq 11\nauth: kk 22\nbilling: bb 33"
	f.responses["tmux list-windows"] = "@1\tzsh\t\n@2\tauth\tauth\n@3\tbilling\tbilling"
	// Both features are done, and we are "inside" auth's window/tag, which
	// should be removed last (killing its window ends this process).
	f.responses["tmux display-message"] = "auth"
	for _, wid := range []string{"@2", "@3"} {
		if err := agentstate.Write(f.app.StateDir, agentstate.Entry{WindowID: wid, Status: agentstate.Done, UpdatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.app.RemoveAllDone(false); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	authIdx := strings.Index(out, `removing "auth"`)
	billingIdx := strings.Index(out, `removing "billing"`)
	if authIdx < 0 || billingIdx < 0 || authIdx < billingIdx {
		t.Errorf("expected the current feature (auth) removed last:\n%s", out)
	}
}

func TestMoveToEnd(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		move  string
		want  []string
	}{
		{name: "present in middle", names: []string{"a", "b", "c"}, move: "b", want: []string{"a", "c", "b"}},
		{name: "already last", names: []string{"a", "b"}, move: "b", want: []string{"a", "b"}},
		{name: "not present", names: []string{"a", "b"}, move: "z", want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := append([]string(nil), tt.names...)
			moveToEnd(got, tt.move)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("moveToEnd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveCannotInfer(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Remove("", false); err == nil || !strings.Contains(err.Error(), "cannot infer") {
		t.Fatalf("got %v", err)
	}
}

func TestListShowsDeadAgent(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.responses["tmux list-windows"] = "@2\tauth\tauth\t1"
	if err := f.app.List(); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "dead") || !strings.Contains(out, "jumux restart auth") {
		t.Errorf("expected dead-agent hint for auth: %s", out)
	}
}

func TestRestartRespawnsDeadPaneAndResendsAgent(t *testing.T) {
	f := newFixture(t)
	f.responses["tmux list-windows"] = "@2\tauth\tauth\t1"
	if err := f.app.Restart("auth", false); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t,
		"tmux respawn-pane -k -t @2",
		"tmux send-keys -t @2 -l claude",
		"tmux send-keys -t @2 Enter",
	)
}

func TestRestartAsksConfirmationForLivePane(t *testing.T) {
	f := newFixture(t)
	// Default fixture window @2 is not marked dead.
	f.app.In = strings.NewReader("n\n")
	if err := f.app.Restart("auth", false); err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected abort, got %v", err)
	}
	f.assertNotRan(t, "respawn-pane")

	f2 := newFixture(t)
	if err := f2.app.Restart("auth", true); err != nil {
		t.Fatal(err)
	}
	f2.assertRan(t, "tmux respawn-pane -k -t @2")
}

func TestRestartUnknownFeature(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Restart("ghost", true); err == nil || !strings.Contains(err.Error(), "no tmux window found") {
		t.Fatalf("got %v", err)
	}
}

func TestRenameHappyPath(t *testing.T) {
	f := newFixture(t)
	ws := f.wsPath("auth")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Rename("auth", "billing"); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t,
		"jj workspace rename billing",
		"tmux rename-window -t @2 billing",
		"tmux set-option -w -t @2 @jumux-feature billing",
	)
	if _, err := os.Stat(ws); !os.IsNotExist(err) {
		t.Error("old workspace dir should no longer exist")
	}
	if _, err := os.Stat(f.wsPath("billing")); err != nil {
		t.Errorf("new workspace dir should exist: %v", err)
	}
}

func TestRenameRunsJJInOldWorkspaceDir(t *testing.T) {
	f := newFixture(t)
	ws := f.wsPath("auth")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Rename("auth", "billing"); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.runner.Calls {
		if c.String() == "jj workspace rename billing" && c.Dir != ws {
			t.Errorf("jj workspace rename ran in %q, want %q", c.Dir, ws)
		}
	}
}

func TestRenameRejectsUnknownOld(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Rename("ghost", "billing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v", err)
	}
}

func TestRenameRejectsExistingNew(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Rename("auth", "auth"); err == nil {
		t.Fatal("expected error for same name")
	}
	f.responses["jj workspace list"] = "default: qq 11\nauth: kk 22\nbilling: mm 33"
	if err := f.app.Rename("auth", "billing"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("got %v", err)
	}
}

func TestRenameRejectsDefault(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Rename("default", "billing"); err == nil || !strings.Contains(err.Error(), "default") {
		t.Fatalf("got %v", err)
	}
	if err := f.app.Rename("auth", "default"); err == nil || !strings.Contains(err.Error(), "default") {
		t.Fatalf("got %v", err)
	}
}

func TestRenameWithoutWindowSkipsTmuxRename(t *testing.T) {
	f := newFixture(t)
	ws := f.wsPath("auth")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	f.responses["tmux list-windows"] = "@1\tzsh\t"
	if err := f.app.Rename("auth", "billing"); err != nil {
		t.Fatal(err)
	}
	f.assertNotRan(t, "rename-window")
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

func TestListShowsRepoColumnWhenAnotherRepoOpen(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.responses["jj workspace list"] = "default: qq 11\nauth: kk 22"
	// A second, unrelated repo's workspace is open in another session.
	otherRepo := filepath.Join(t.TempDir(), "otherrepo")
	if err := os.MkdirAll(filepath.Join(otherRepo, ".jj", "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.responses["tmux list-windows -a"] = "$0\tmain\t@2\tauth\tauth\t" + f.wsPath("auth") + "\t0\n" +
		"$1\tother\t@8\tfoo\tfoo\t" + otherRepo + "\t0"
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "tmux list-windows -a"):
			return f.responses["tmux list-windows -a"], nil
		case strings.HasPrefix(cmd, "jj root"):
			return dir, nil // jj.Root resolves to the workspace containing dir
		}
		for prefix, resp := range f.responses {
			if strings.HasPrefix(cmd, prefix) {
				return resp, nil
			}
		}
		return "", nil
	}
	if err := f.app.List(); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "REPO") {
		t.Errorf("expected a REPO column header when another repo is open: %s", out)
	}
	if !strings.Contains(out, "myrepo") {
		t.Errorf("expected rows labeled with the current repo name: %s", out)
	}
}

func TestListOmitsRepoColumnForSingleRepo(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.responses["jj workspace list"] = "default: qq 11\nauth: kk 22"
	if err := f.app.List(); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if strings.Contains(out, "REPO") {
		t.Errorf("expected no REPO column with a single repo open: %s", out)
	}
}

func TestListSurfacesIdleFeatures(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(f.wsPath("fresh"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.responses["tmux list-windows"] = "@1\tzsh\t\n@2\tauth\tauth\n@3\tfresh\tfresh"
	f.responses["jj workspace list"] = "default: qq 11\nauth: kk 22\nfresh: ff 33"
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	f.app.Now = func() time.Time { return now }
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case strings.Contains(cmd, "committer.timestamp"):
			switch {
			case strings.Contains(cmd, "auth@"):
				return fmt.Sprintf("%d", now.Add(-240*time.Hour).Unix()), nil // 10d old
			case strings.Contains(cmd, "fresh@"):
				return fmt.Sprintf("%d", now.Add(-time.Hour).Unix()), nil
			}
			return "", errors.New("no timestamp")
		default:
			for prefix, resp := range f.responses {
				if strings.HasPrefix(cmd, prefix) {
					return resp, nil
				}
			}
			return "", nil
		}
	}
	if err := f.app.List(); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	lines := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			lines[fields[0]] = line
		}
	}
	if !strings.Contains(lines["auth"], "(idle)") {
		t.Errorf("auth row should be marked idle: %q", lines["auth"])
	}
	if strings.Contains(lines["fresh"], "(idle)") {
		t.Errorf("fresh row should not be marked idle: %q", lines["fresh"])
	}
}
