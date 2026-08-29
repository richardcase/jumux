package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

// useSecondaryWorkspace points the fixture's cwd at a feature's secondary jj
// workspace: a directory with a .jj/repo *file* pointing back at the main
// repo store, and deliberately no .git. It returns the workspace root.
func useSecondaryWorkspace(t *testing.T, f *fixture, feature string) string {
	t.Helper()
	wsRoot := f.wsPath(feature)
	if err := os.MkdirAll(filepath.Join(wsRoot, ".jj"), 0o755); err != nil {
		t.Fatal(err)
	}
	pointer := filepath.Join(f.mainRoot, ".jj", "repo")
	if err := os.WriteFile(filepath.Join(wsRoot, ".jj", "repo"), []byte(pointer), 0o644); err != nil {
		t.Fatal(err)
	}
	f.responses["jj root"] = wsRoot
	f.app.Getwd = func() (string, error) { return wsRoot, nil }
	return wsRoot
}

// execStyleError formats an error the way run.ExecRunner does, folding the
// whole argv — title and body included — into the message.
func execStyleError(name string, args []string, stderr string) error {
	return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "),
		errors.New("exit status 1"), stderr)
}

// hostCall returns the single call made to the host CLI binary.
func hostCall(t *testing.T, f *fixture, bin string) run.Call {
	t.Helper()
	var found []run.Call
	for _, c := range f.runner.Calls {
		if c.Name == bin {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one %s call, got %d; ran:\n%s", bin, len(found), f.runner.CommandLines())
	}
	return found[0]
}

func TestPRExplicitFeature(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = "Add widget support\n\nThis adds widgets."

	if err := f.app.PR("myfeature"); err != nil {
		t.Fatalf("PR() error = %v", err)
	}

	c := hostCall(t, f, "gh")
	wantArgs := []string{"pr", "create", "--head", "myfeature",
		"--title", "Add widget support", "--body", "This adds widgets."}
	if strings.Join(c.Args, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("gh args = %v, want %v", c.Args, wantArgs)
	}
}

// gh resolves the git remote, so it must run in the colocated main
// workspace, never in the feature's secondary workspace (no .git there).
func TestPRRunsGHInMainRoot(t *testing.T) {
	f := newFixture(t)
	wsRoot := useSecondaryWorkspace(t, f, "auth")
	f.responses["jj log"] = "Add auth"

	if err := f.app.PR("auth"); err != nil {
		t.Fatalf("PR() error = %v", err)
	}

	c := hostCall(t, f, "gh")
	if c.Dir != f.mainRoot {
		t.Errorf("gh ran in %q, want main root %q", c.Dir, f.mainRoot)
	}
	if c.Dir == wsRoot {
		t.Errorf("gh ran in the secondary workspace %q, which has no .git", wsRoot)
	}
}

func TestPRInfersFeatureFromWindowTag(t *testing.T) {
	f := newFixture(t)
	f.responses["tmux display-message"] = "auth"
	f.responses["jj log"] = "Add widget support\n\nThis adds widgets."

	if err := f.app.PR(""); err != nil {
		t.Fatalf("PR() error = %v", err)
	}

	f.assertRan(t, "jj bookmark set auth -r auth@", "jj git push --bookmark auth")

	c := hostCall(t, f, "gh")
	wantArgs := []string{"pr", "create", "--head", "auth",
		"--title", "Add widget support", "--body", "This adds widgets."}
	if strings.Join(c.Args, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("gh args = %v, want %v", c.Args, wantArgs)
	}
}

func TestPRAlreadyExistsSucceeds(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = "Add widget support"
	orig := f.runner.Handler
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "gh" {
			return "", execStyleError(name, args,
				`a pull request for branch "myfeature" into branch "main" already exists`)
		}
		return orig(dir, name, args...)
	}

	if err := f.app.PR("myfeature"); err != nil {
		t.Fatalf("PR() error = %v, want nil (already-exists should succeed)", err)
	}
}

// The formatted error from run.ExecRunner echoes the title back, so a
// failure whose title happens to contain the already-exists marker must
// still be reported as a failure.
func TestPRFailureWithMarkerInTitleIsError(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = `Fix the "a pull request for branch X already exists" handling`
	orig := f.runner.Handler
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "gh" {
			return "", execStyleError(name, args, "authentication required")
		}
		return orig(dir, name, args...)
	}

	if err := f.app.PR("myfeature"); err == nil {
		t.Fatal("expected an error for an auth failure, got nil")
	}
}

func TestPRGhFailureIsError(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = "Add widget support"
	orig := f.runner.Handler
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "gh" {
			return "", errors.New("gh: command not found")
		}
		return orig(dir, name, args...)
	}

	if err := f.app.PR("myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPREmptyDescriptionIsError(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = ""

	err := f.app.PR("myfeature")
	if err == nil {
		t.Fatal("expected an error for a missing change description, got nil")
	}
	if !strings.Contains(err.Error(), "no change description") {
		t.Errorf("error = %v, want it to mention the missing description", err)
	}
	f.assertNotRan(t, "gh pr create")
}

func TestPRRejectsInvalidFeatureNames(t *testing.T) {
	tests := []struct {
		name    string
		feature string
	}{
		{name: "default workspace", feature: "default"},
		{name: "path traversal", feature: "../etc"},
		{name: "leading dot", feature: ".hidden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.responses["jj log"] = "Add widget support"
			if err := f.app.PR(tt.feature); err == nil {
				t.Fatalf("PR(%q) error = nil, want an error", tt.feature)
			}
			f.assertNotRan(t, "gh")
			f.assertNotRan(t, "jj git push")
		})
	}
}
