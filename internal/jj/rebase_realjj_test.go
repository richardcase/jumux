package jj

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

// TestRebasePreservesFeatureStack is a regression test for the bug where
// `jj rebase -r <rev> -d <dest>` moved only the working-copy commit, leaving
// earlier feature commits behind on the old base. It exercises the real jj
// binary (skipped if not installed) because the fake-runner tests only
// assert the command string, not what jj actually does to the commit graph.
func TestRebasePreservesFeatureStack(t *testing.T) {
	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj binary not installed")
	}
	r := run.ExecRunner{}
	repo := t.TempDir()
	runjj := func(dir string, args ...string) string {
		t.Helper()
		out, err := r.Run(dir, "jj", args...)
		if err != nil {
			t.Fatalf("jj %s: %v", strings.Join(args, " "), err)
		}
		return out
	}
	writeFile := func(dir, name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	runjj(repo, "git", "init", "--colocate")
	writeFile(repo, "base.txt")
	runjj(repo, "commit", "-m", "base")
	baseID := runjj(repo, "log", "-r", "@-", "--no-graph", "-T", "commit_id")

	// Feature workspace with two commits on top of base.
	wsPath := filepath.Join(t.TempDir(), "auth")
	runjj(repo, "workspace", "add", "--name", "auth", "-r", baseID, wsPath)
	writeFile(wsPath, "feature-first.txt")
	runjj(wsPath, "commit", "-m", "feature-first")
	writeFile(wsPath, "feature-second.txt")
	runjj(wsPath, "commit", "-m", "feature-second")

	// Advance the base independently of the feature stack.
	runjj(repo, "new", baseID, "-m", "updated-main")
	writeFile(repo, "updated-main.txt")
	runjj(repo, "bookmark", "create", "main", "-r", "@")

	if err := Rebase(r, wsPath, "auth@", "main"); err != nil {
		t.Fatalf("Rebase() error = %v", err)
	}

	for _, f := range []string{"feature-first.txt", "feature-second.txt", "updated-main.txt"} {
		if _, err := os.Stat(filepath.Join(wsPath, f)); err != nil {
			t.Errorf("expected %s to survive the rebase: %v", f, err)
		}
	}

	ancestry := runjj(wsPath, "log", "-r", "::auth@", "--no-graph", "-T", `description ++ "\n"`)
	for _, want := range []string{"feature-first", "feature-second", "updated-main"} {
		if !strings.Contains(ancestry, want) {
			t.Errorf("expected auth@'s ancestry to contain %q, got:\n%s", want, ancestry)
		}
	}
}
