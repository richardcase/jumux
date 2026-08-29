package jj

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

func TestWorkspacesParsing(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "default: qpvuntsm 4f9c4dc4 (empty) (no description set)\nauth: kxxprrqz 12ab34cd feat: login", nil
	}}
	names, err := Workspaces(fr, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"default", "auth"}; !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestMainRoot(t *testing.T) {
	tmp := t.TempDir()
	mainRoot := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(filepath.Join(mainRoot, ".jj", "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Main workspace: .jj/repo is a directory.
	got, err := MainRoot(mainRoot)
	if err != nil || got != mainRoot {
		t.Errorf("main workspace: got %q, %v", got, err)
	}
	// Secondary workspace: .jj/repo is a file pointing at the store.
	ws := filepath.Join(tmp, "repo-auth")
	if err := os.MkdirAll(filepath.Join(ws, ".jj"), 0o755); err != nil {
		t.Fatal(err)
	}
	pointer := filepath.Join(mainRoot, ".jj", "repo")
	if err := os.WriteFile(filepath.Join(ws, ".jj", "repo"), []byte(pointer), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = MainRoot(ws)
	if err != nil || got != mainRoot {
		t.Errorf("secondary workspace (absolute pointer): got %q, %v", got, err)
	}
	// Some jj versions write the pointer relative to the workspace's .jj dir.
	if err := os.WriteFile(filepath.Join(ws, ".jj", "repo"), []byte("../../repo/.jj/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = MainRoot(ws)
	if err != nil || got != mainRoot {
		t.Errorf("secondary workspace (relative pointer): got %q, %v", got, err)
	}
}

func TestInstalled(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "jj 0.20.0", nil
	}}
	if err := Installed(fr); err != nil {
		t.Errorf("got %v, want nil", err)
	}

	frErr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", errors.New("exec: \"jj\": executable file not found in $PATH")
	}}
	if err := Installed(frErr); err == nil {
		t.Error("got nil error, want non-nil when jj is not installed")
	}
}

func TestResolveRevision(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", nil
	}}
	if err := ResolveRevision(fr, "/repo", "trunk()"); err != nil {
		t.Errorf("got %v, want nil", err)
	}
	if got := fr.Calls[0].Dir; got != "/repo" {
		t.Errorf("resolve must run in the given dir, ran in %q", got)
	}

	frErr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", errors.New("revset \"nope\" doesn't resolve to any revisions")
	}}
	if err := ResolveRevision(frErr, "/repo", "nope"); err == nil {
		t.Error("got nil error, want non-nil for an unresolvable revset")
	}
}

func TestIsDirty(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "dirty", nil
	}}
	dirty, err := IsDirty(fr, "/ws", "auth")
	if err != nil || !dirty {
		t.Errorf("got %v, %v", dirty, err)
	}
	if got := fr.Calls[0].Dir; got != "/ws" {
		t.Errorf("dirty check must run inside the workspace, ran in %q", got)
	}
}

func TestBookmarkSet(t *testing.T) {
	fr := &run.FakeRunner{}
	err := BookmarkSet(fr, "/repo", "myfeature", "myfeature@")
	if err != nil {
		t.Fatalf("BookmarkSet() error = %v", err)
	}
	want := "jj bookmark set myfeature -r myfeature@\n"
	if got := fr.CommandLines(); got != want {
		t.Errorf("CommandLines() = %q, want %q", got, want)
	}
}

func TestBookmarkSetError(t *testing.T) {
	frErr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", errors.New("boom")
	}}
	if err := BookmarkSet(frErr, "/repo", "myfeature", "myfeature@"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGitPush(t *testing.T) {
	fr := &run.FakeRunner{}
	err := GitPush(fr, "/repo", "myfeature")
	if err != nil {
		t.Fatalf("GitPush() error = %v", err)
	}
	want := "jj git push --bookmark myfeature\n"
	if got := fr.CommandLines(); got != want {
		t.Errorf("CommandLines() = %q, want %q", got, want)
	}
}

func TestGitPushError(t *testing.T) {
	frErr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", errors.New("boom")
	}}
	if err := GitPush(frErr, "/repo", "myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDescription(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "Add widget support\n\nThis adds widgets.", nil
	}}
	got, err := Description(fr, "/repo", "myfeature@")
	if err != nil {
		t.Fatalf("Description() error = %v", err)
	}
	want := "Add widget support\n\nThis adds widgets."
	if got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
	wantCmd := "jj log -r myfeature@ --no-graph -T description\n"
	if got := fr.CommandLines(); got != wantCmd {
		t.Errorf("CommandLines() = %q, want %q", got, wantCmd)
	}
}

func TestDescriptionEmpty(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", nil
	}}
	got, err := Description(fr, "/repo", "myfeature@")
	if err != nil {
		t.Fatalf("Description() error = %v", err)
	}
	if got != "" {
		t.Errorf("Description() = %q, want empty", got)
	}
}

func TestDescriptionError(t *testing.T) {
	frErr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", errors.New("boom")
	}}
	if _, err := Description(frErr, "/repo", "myfeature@"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
