package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathPrintsWorkspacePathForExplicitFeature(t *testing.T) {
	f := newFixture(t)
	ws := f.wsPath("auth")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := f.app.Path("auth"); err != nil {
		t.Fatal(err)
	}

	want := ws + "\n"
	if got := f.out.String(); got != want {
		t.Errorf("Path(\"auth\") = %q, want %q", got, want)
	}
}

func TestPathPrintsDefaultWorkspacePath(t *testing.T) {
	f := newFixture(t)

	if err := f.app.Path("default"); err != nil {
		t.Fatal(err)
	}

	want := f.mainRoot + "\n"
	if got := f.out.String(); got != want {
		t.Errorf("Path(\"default\") = %q, want %q", got, want)
	}
}

func TestPathInfersFeatureFromCurrentWorkspace(t *testing.T) {
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

	if err := f.app.Path(""); err != nil {
		t.Fatal(err)
	}

	want := ws + "\n"
	if got := f.out.String(); got != want {
		t.Errorf("Path(\"\") = %q, want %q", got, want)
	}
}

func TestPathInfersFeatureFromTmuxWindow(t *testing.T) {
	f := newFixture(t)
	ws := f.wsPath("auth")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	f.responses["tmux display-message"] = "auth"

	if err := f.app.Path(""); err != nil {
		t.Fatal(err)
	}

	want := ws + "\n"
	if got := f.out.String(); got != want {
		t.Errorf("Path(\"\") = %q, want %q", got, want)
	}
}

func TestPathFailsWhenFeatureNotFound(t *testing.T) {
	f := newFixture(t)

	err := f.app.Path("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent workspace, got nil")
	}
	if !strings.Contains(err.Error(), `workspace "nonexistent" not found`) {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestPathFailsWhenDirectoryMissing(t *testing.T) {
	f := newFixture(t)
	// "auth" is in workspace list, but directory has not been created on disk

	err := f.app.Path("auth")
	if err == nil {
		t.Fatal("expected error for missing workspace directory, got nil")
	}
	if !strings.Contains(err.Error(), `workspace "auth" not found at`) {
		t.Errorf("expected directory not found error, got %v", err)
	}
}

func TestPathFailsOnInvalidFeatureName(t *testing.T) {
	f := newFixture(t)

	err := f.app.Path("invalid/name")
	if err == nil {
		t.Fatal("expected error for invalid feature name, got nil")
	}
	if !strings.Contains(err.Error(), "invalid feature name") {
		t.Errorf("expected invalid feature name error, got %v", err)
	}
}

func TestPathWorksOutsideTmux(t *testing.T) {
	f := newFixture(t)
	ws := f.wsPath("auth")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	// Simulate running outside tmux
	origGetenv := f.app.Getenv
	f.app.Getenv = func(k string) string {
		if k == "TMUX" {
			return ""
		}
		return origGetenv(k)
	}

	if err := f.app.Path("auth"); err != nil {
		t.Fatalf("expected Path to work outside tmux, got %v", err)
	}

	want := ws + "\n"
	if got := f.out.String(); got != want {
		t.Errorf("Path(\"auth\") = %q, want %q", got, want)
	}
}
