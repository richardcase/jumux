package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeHappyPath(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Merge("auth", false); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t,
		"jj rebase -b auth -d main",
		"jj bookmark set main -r auth@",
		"jj workspace forget auth",
		"tmux kill-window -t @2",
	)
	if _, err := os.Stat(f.wsPath("auth")); !os.IsNotExist(err) {
		t.Error("workspace dir should be deleted")
	}
}

func TestMergeConflictAbortsCleanup(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := f.runner.Handler
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		for _, a := range args {
			if strings.Contains(a, "if(conflict") {
				return "conflict", nil
			}
		}
		return orig(dir, name, args...)
	}

	err := f.app.Merge("auth", false)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected a conflict error, got %v", err)
	}
	f.assertRan(t, "jj rebase -b auth -d main")
	f.assertNotRan(t, "jj bookmark set", "jj workspace forget", "tmux kill-window")
	if _, err := os.Stat(f.wsPath("auth")); err != nil {
		t.Error("workspace dir should survive a conflicted merge")
	}
}

func TestMergeDirtyDeclinedAndForced(t *testing.T) {
	f := newFixture(t)
	f.responses["jj log"] = "dirty"
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.app.In = strings.NewReader("n\n")
	if err := f.app.Merge("auth", false); err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected abort, got %v", err)
	}
	f.assertNotRan(t, "jj rebase")

	// --force skips the check entirely.
	f2 := newFixture(t)
	f2.responses["jj log"] = "dirty"
	if err := os.MkdirAll(f2.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f2.app.Merge("auth", true); err != nil {
		t.Fatal(err)
	}
	f2.assertRan(t, "jj rebase -b auth -d main", "jj workspace forget auth")
}

func TestMergeRefusesDefault(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Merge("default", false); err == nil || !strings.Contains(err.Error(), "default") {
		t.Fatalf("got %v", err)
	}
}

func TestMergeNothingFound(t *testing.T) {
	f := newFixture(t)
	f.responses["tmux list-windows"] = "@1\tzsh\t"
	if err := f.app.Merge("ghost", false); err == nil || !strings.Contains(err.Error(), "nothing to merge") {
		t.Fatalf("got %v", err)
	}
}

func TestMergeInfersFeatureFromWindowTag(t *testing.T) {
	f := newFixture(t)
	f.responses["tmux display-message"] = "auth"
	if err := f.app.Merge("", false); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "jj rebase -b auth -d main")
}

func TestMergeUsesConfiguredBaseBookmark(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(filepath.Join(f.mainRoot, ".jumux.toml"), []byte(`base_bookmark = "develop"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Merge("auth", false); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "jj rebase -b auth -d develop", "jj bookmark set develop -r auth@")
}

func TestMergeRejectsInvalidFeatureNames(t *testing.T) {
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
			if err := f.app.Merge(tt.feature, false); err == nil {
				t.Fatalf("Merge(%q) error = nil, want an error", tt.feature)
			}
			f.assertNotRan(t, "jj rebase")
		})
	}
}
