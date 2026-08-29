package forge

import (
	"errors"
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

func TestPreparePushEmptyDescriptionFallsBackToFeatureName(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", nil
	}}

	title, body, err := PreparePush(fr, "/repo/ws", "myfeature")
	if err != nil {
		t.Fatalf("PreparePush() error = %v", err)
	}
	if title != "myfeature" {
		t.Errorf("title = %q, want %q", title, "myfeature")
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestPreparePushBookmarkSetError(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 1 && args[0] == "bookmark" {
			return "", errors.New("boom")
		}
		return "", nil
	}}

	if _, _, err := PreparePush(fr, "/repo/ws", "myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPreparePushGitPushError(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 1 && args[0] == "git" {
			return "", errors.New("boom")
		}
		return "", nil
	}}

	if _, _, err := PreparePush(fr, "/repo/ws", "myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPreparePushCommandSequence(t *testing.T) {
	fr := &run.FakeRunner{}
	if _, _, err := PreparePush(fr, "/repo/ws", "myfeature"); err != nil {
		t.Fatalf("PreparePush() error = %v", err)
	}
	want := "jj bookmark set myfeature -r myfeature@\n" +
		"jj git push --bookmark myfeature\n" +
		"jj log -r myfeature@ --no-graph -T description\n"
	if got := fr.CommandLines(); got != want {
		t.Errorf("CommandLines() = %q, want %q", got, want)
	}
}
