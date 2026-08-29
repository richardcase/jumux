package forge

import (
	"errors"
	"strings"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

func TestPreparePushWithDescription(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 0 && args[0] == "log" {
			return "Add widget support\n\nThis adds widgets.", nil
		}
		return "", nil
	}}

	title, body, err := PreparePush(fr, "/repo/ws", "myfeature")
	if err != nil {
		t.Fatalf("PreparePush() error = %v", err)
	}
	if title != "Add widget support" {
		t.Errorf("title = %q, want %q", title, "Add widget support")
	}
	if body != "This adds widgets." {
		t.Errorf("body = %q, want %q", body, "This adds widgets.")
	}
}

// jj refuses to push a bookmark on a description-less commit, so PreparePush
// must reject that itself, before touching the bookmark or pushing.
func TestPreparePushEmptyDescriptionIsAnError(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "   \n", nil
	}}

	_, _, err := PreparePush(fr, "/repo/ws", "myfeature")
	if err == nil {
		t.Fatal("expected an error for an empty description, got nil")
	}
	if !strings.Contains(err.Error(), "no change description") {
		t.Errorf("error = %v, want it to mention the missing description", err)
	}
	if got := fr.CommandLines(); strings.Contains(got, "bookmark") || strings.Contains(got, "push") {
		t.Errorf("expected no bookmark/push after an empty description; ran:\n%s", got)
	}
}

// described returns a runner whose jj log reports a description, so
// PreparePush gets past its description check.
func described(fail func(dir, name string, args ...string) (string, error)) *run.FakeRunner {
	return &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 0 && args[0] == "log" {
			return "Add widget support", nil
		}
		if fail != nil {
			return fail(dir, name, args...)
		}
		return "", nil
	}}
}

func TestPreparePushBookmarkSetError(t *testing.T) {
	fr := described(func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 1 && args[0] == "bookmark" {
			return "", errors.New("boom")
		}
		return "", nil
	})

	if _, _, err := PreparePush(fr, "/repo/ws", "myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPreparePushGitPushError(t *testing.T) {
	fr := described(func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 1 && args[0] == "git" {
			return "", errors.New("boom")
		}
		return "", nil
	})

	if _, _, err := PreparePush(fr, "/repo/ws", "myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPreparePushCommandSequence(t *testing.T) {
	fr := described(nil)
	if _, _, err := PreparePush(fr, "/repo/ws", "myfeature"); err != nil {
		t.Fatalf("PreparePush() error = %v", err)
	}
	want := "jj log -r myfeature@ --no-graph -T description\n" +
		"jj bookmark set myfeature -r myfeature@\n" +
		"jj git push --bookmark myfeature\n"
	if got := fr.CommandLines(); got != want {
		t.Errorf("CommandLines() = %q, want %q", got, want)
	}
	for _, c := range fr.Calls {
		if c.Dir != "/repo/ws" {
			t.Errorf("call %v ran in %q, want %q", c, c.Dir, "/repo/ws")
		}
	}
}
