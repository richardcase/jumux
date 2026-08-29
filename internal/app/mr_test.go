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

	found := false
	for _, c := range f.runner.Calls {
		if c.Name == "glab" && len(c.Args) > 0 && c.Args[0] == "mr" {
			found = true
			wantArgs := []string{"mr", "create", "--title", "Add widget support", "--description", "This adds widgets."}
			if strings.Join(c.Args, " ") != strings.Join(wantArgs, " ") {
				t.Errorf("glab args = %v, want %v", c.Args, wantArgs)
			}
		}
	}
	if !found {
		t.Error("expected a glab mr create call, got none")
	}
}

func TestMRAlreadyExistsSucceeds(t *testing.T) {
	f := newFixture(t)
	orig := f.runner.Handler
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "glab" {
			return "", errors.New("merge request already exists")
		}
		return orig(dir, name, args...)
	}

	if err := f.app.MR("myfeature"); err != nil {
		t.Fatalf("MR() error = %v, want nil (already-exists should succeed)", err)
	}
}

func TestMRGlabFailureIsError(t *testing.T) {
	f := newFixture(t)
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
