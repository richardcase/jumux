package app

import (
	"errors"
	"strings"
	"testing"
)

func TestPRExplicitFeature(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = "Add widget support\n\nThis adds widgets."

	if err := f.app.PR("myfeature"); err != nil {
		t.Fatalf("PR() error = %v", err)
	}

	found := false
	for _, c := range f.runner.Calls {
		if c.Name == "gh" && len(c.Args) > 0 && c.Args[0] == "pr" {
			found = true
			wantArgs := []string{"pr", "create", "--title", "Add widget support", "--body", "This adds widgets."}
			if strings.Join(c.Args, " ") != strings.Join(wantArgs, " ") {
				t.Errorf("gh args = %v, want %v", c.Args, wantArgs)
			}
		}
	}
	if !found {
		t.Error("expected a gh pr create call, got none")
	}
}

func TestPRAlreadyExistsSucceeds(t *testing.T) {
	f := newFixture(t)
	orig := f.runner.Handler
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "gh" {
			return "", errors.New("a pull request for branch \"myfeature\" into branch \"main\" already exists")
		}
		return orig(dir, name, args...)
	}

	if err := f.app.PR("myfeature"); err != nil {
		t.Fatalf("PR() error = %v, want nil (already-exists should succeed)", err)
	}
}

func TestPRGhFailureIsError(t *testing.T) {
	f := newFixture(t)
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
