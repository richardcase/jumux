package jj

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/richardcase/agentmux/internal/run"
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
