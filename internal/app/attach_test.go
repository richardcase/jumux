package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

// attachTwoRepoFixture builds two independent main repos, each with a
// workspace for the same feature name, so tests can verify Attach only
// considers windows belonging to the caller's repo.
func attachTwoRepoFixture(t *testing.T, feature string) (repoA, repoB, wsA, wsB string) {
	t.Helper()
	tmp := t.TempDir()
	repoA = filepath.Join(tmp, "repoA")
	repoB = filepath.Join(tmp, "repoB")
	wsA = filepath.Join(tmp, "repoA-"+feature)
	wsB = filepath.Join(tmp, "repoB-"+feature)
	for _, root := range []string{repoA, repoB} {
		if err := os.MkdirAll(filepath.Join(root, ".jj", "repo"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for ws, mainRoot := range map[string]string{wsA: repoA, wsB: repoB} {
		if err := os.MkdirAll(filepath.Join(ws, ".jj"), 0o755); err != nil {
			t.Fatal(err)
		}
		pointer := filepath.Join(mainRoot, ".jj", "repo")
		if err := os.WriteFile(filepath.Join(ws, ".jj", "repo"), []byte(pointer), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repoA, repoB, wsA, wsB
}

func newAttachApp(runner *run.FakeRunner, cwd string) *App {
	return &App{
		Runner: runner,
		Out:    &bytes.Buffer{},
		Errw:   &bytes.Buffer{},
		Getwd:  func() (string, error) { return cwd, nil },
		Getenv: func(k string) string {
			if k == "TMUX" {
				return "/tmp/tmux-1/default,123,0"
			}
			return ""
		},
	}
}

func TestAttachIgnoresSameFeatureWindowInOtherRepo(t *testing.T) {
	repoA, _, wsA, wsB := attachTwoRepoFixture(t, "auth")
	runner := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "jj root"):
			return dir, nil
		case strings.HasPrefix(cmd, "tmux list-windows") && strings.Contains(cmd, "-a"):
			// repoB's window is listed first, to prove Attach doesn't just
			// take the first tmux match across repos.
			return "$1\tother\t@9\tam-auth\tauth\t" + wsB + "\t0\t0\n" +
				"$0\tmain\t@2\tam-auth\tauth\t" + wsA + "\t0\t0", nil
		}
		return "", nil
	}}
	app := newAttachApp(runner, repoA)
	if err := app.Attach("auth"); err != nil {
		t.Fatal(err)
	}
	lines := runner.CommandLines()
	if !strings.Contains(lines, "select-window -t @2") || !strings.Contains(lines, "switch-client -t $0") {
		t.Errorf("expected attach to repoA's window (@2/$0); ran:\n%s", lines)
	}
	if strings.Contains(lines, "-t @9") || strings.Contains(lines, "switch-client -t $1") {
		t.Errorf("attach must not select repoB's window; ran:\n%s", lines)
	}
}

func TestAttachErrorsWhenOnlyOtherRepoHasMatchingWindow(t *testing.T) {
	repoA, _, _, wsB := attachTwoRepoFixture(t, "auth")
	runner := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "jj root"):
			return dir, nil
		case strings.HasPrefix(cmd, "tmux list-windows") && strings.Contains(cmd, "-a"):
			return "$1\tother\t@9\tam-auth\tauth\t" + wsB + "\t0\t0", nil
		}
		return "", nil
	}}
	app := newAttachApp(runner, repoA)
	err := app.Attach("auth")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "no tmux window found for feature \"auth\"") ||
		!strings.Contains(err.Error(), "this repository") {
		t.Errorf("expected a repo-scoped not-found error, got: %v", err)
	}
}
