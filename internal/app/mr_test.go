package app

import (
	"errors"
	"strings"
	"testing"
)

func TestMRExplicitFeature(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = "Add widget support\n\nThis adds widgets."

	if err := f.app.MR("myfeature"); err != nil {
		t.Fatalf("MR() error = %v", err)
	}

	c := hostCall(t, f, "glab")
	wantArgs := []string{"mr", "create", "--source-branch", "myfeature",
		"--title", "Add widget support", "--description", "This adds widgets."}
	if strings.Join(c.Args, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("glab args = %v, want %v", c.Args, wantArgs)
	}
}

// glab resolves the git remote, so it must run in the colocated main
// workspace, never in the feature's secondary workspace (no .git there).
func TestMRRunsGlabInMainRoot(t *testing.T) {
	f := newFixture(t)
	wsRoot := useSecondaryWorkspace(t, f, "auth")
	f.responses["jj log"] = "Add auth"

	if err := f.app.MR("auth"); err != nil {
		t.Fatalf("MR() error = %v", err)
	}

	c := hostCall(t, f, "glab")
	if c.Dir != f.mainRoot {
		t.Errorf("glab ran in %q, want main root %q", c.Dir, f.mainRoot)
	}
	if c.Dir == wsRoot {
		t.Errorf("glab ran in the secondary workspace %q, which has no .git", wsRoot)
	}
}

func TestMRInfersFeatureFromWindowTag(t *testing.T) {
	f := newFixture(t)
	f.responses["tmux display-message"] = "auth"
	f.responses["jj log"] = "Add widget support\n\nThis adds widgets."

	if err := f.app.MR(""); err != nil {
		t.Fatalf("MR() error = %v", err)
	}

	f.assertRan(t, "jj bookmark set auth -r auth@", "jj git push --bookmark auth")

	c := hostCall(t, f, "glab")
	wantArgs := []string{"mr", "create", "--source-branch", "auth",
		"--title", "Add widget support", "--description", "This adds widgets."}
	if strings.Join(c.Args, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("glab args = %v, want %v", c.Args, wantArgs)
	}
}

func TestMRAlreadyExistsSucceeds(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = "Add widget support"
	orig := f.runner.Handler
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "glab" {
			return "", execStyleError(name, args, "merge request already exists")
		}
		return orig(dir, name, args...)
	}

	if err := f.app.MR("myfeature"); err != nil {
		t.Fatalf("MR() error = %v, want nil (already-exists should succeed)", err)
	}
}

// The formatted error from run.ExecRunner echoes the body back, so a
// failure whose body happens to contain the already-exists marker must
// still be reported as a failure.
func TestMRFailureWithMarkerInBodyIsError(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = "Add widget support\n\nNote: the upstream tag already exists."
	orig := f.runner.Handler
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "glab" {
			return "", execStyleError(name, args, "authentication required")
		}
		return orig(dir, name, args...)
	}

	if err := f.app.MR("myfeature"); err == nil {
		t.Fatal("expected an error for an auth failure, got nil")
	}
}

func TestMRGlabFailureIsError(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = "Add widget support"
	orig := f.runner.Handler
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "glab" {
			return "", errors.New("glab: command not found")
		}
		return orig(dir, name, args...)
	}

	if err := f.app.MR("myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMREmptyDescriptionIsError(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = ""

	err := f.app.MR("myfeature")
	if err == nil {
		t.Fatal("expected an error for a missing change description, got nil")
	}
	if !strings.Contains(err.Error(), "no change description") {
		t.Errorf("error = %v, want it to mention the missing description", err)
	}
	f.assertNotRan(t, "glab mr create")
}

func TestMRRejectsInvalidFeatureNames(t *testing.T) {
	for _, feature := range []string{"default", "../etc", ".hidden"} {
		t.Run(feature, func(t *testing.T) {
			f := newFixture(t)
			f.responses["jj log"] = "Add widget support"
			if err := f.app.MR(feature); err == nil {
				t.Fatalf("MR(%q) error = nil, want an error", feature)
			}
			f.assertNotRan(t, "glab")
			f.assertNotRan(t, "jj git push")
		})
	}
}
