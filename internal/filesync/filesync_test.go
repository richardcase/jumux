package filesync

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyCopiesFile(t *testing.T) {
	mainRoot := t.TempDir()
	wsPath := t.TempDir()

	if err := os.WriteFile(filepath.Join(mainRoot, ".env"), []byte("SECRET=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	Apply(mainRoot, wsPath, []string{".env"}, nil, &warn)

	got, err := os.ReadFile(filepath.Join(wsPath, ".env"))
	if err != nil {
		t.Fatalf("reading copied file: %v", err)
	}
	if string(got) != "SECRET=1\n" {
		t.Errorf("copied content = %q, want %q", got, "SECRET=1\n")
	}
	if warn.Len() != 0 {
		t.Errorf("unexpected warnings: %s", warn.String())
	}
}

func TestApplyCopiesDirectoryRecursively(t *testing.T) {
	mainRoot := t.TempDir()
	wsPath := t.TempDir()

	if err := os.MkdirAll(filepath.Join(mainRoot, "config", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainRoot, "config", "local.toml"), []byte("a=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainRoot, "config", "nested", "b.toml"), []byte("b=2"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	Apply(mainRoot, wsPath, []string{"config"}, nil, &warn)

	for _, rel := range []string{filepath.Join("config", "local.toml"), filepath.Join("config", "nested", "b.toml")} {
		if _, err := os.Stat(filepath.Join(wsPath, rel)); err != nil {
			t.Errorf("expected %s to be copied: %v", rel, err)
		}
	}
	if warn.Len() != 0 {
		t.Errorf("unexpected warnings: %s", warn.String())
	}
}

func TestApplySymlinksFile(t *testing.T) {
	mainRoot := t.TempDir()
	wsPath := t.TempDir()

	src := filepath.Join(mainRoot, ".env")
	if err := os.WriteFile(src, []byte("SECRET=1"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	Apply(mainRoot, wsPath, nil, []string{".env"}, &warn)

	dst := filepath.Join(wsPath, ".env")
	target, err := os.Readlink(dst)
	if err != nil {
		t.Fatalf("reading symlink: %v", err)
	}
	if target != src {
		t.Errorf("symlink target = %q, want %q", target, src)
	}
	if warn.Len() != 0 {
		t.Errorf("unexpected warnings: %s", warn.String())
	}
}

func TestApplySymlinksDirectory(t *testing.T) {
	mainRoot := t.TempDir()
	wsPath := t.TempDir()

	src := filepath.Join(mainRoot, "node_modules")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	Apply(mainRoot, wsPath, nil, []string{"node_modules"}, &warn)

	dst := filepath.Join(wsPath, "node_modules")
	target, err := os.Readlink(dst)
	if err != nil {
		t.Fatalf("reading symlink: %v", err)
	}
	if target != src {
		t.Errorf("symlink target = %q, want %q", target, src)
	}
}

func TestApplySkipsMissingSourceSilently(t *testing.T) {
	mainRoot := t.TempDir()
	wsPath := t.TempDir()

	var warn bytes.Buffer
	Apply(mainRoot, wsPath, []string{".env"}, []string{"node_modules"}, &warn)

	if warn.Len() != 0 {
		t.Errorf("expected no warnings for missing (optional) sources, got: %s", warn.String())
	}
}

func TestApplyRejectsPathTraversalInCopy(t *testing.T) {
	parent := t.TempDir()
	mainRoot := filepath.Join(parent, "repo")
	wsPath := filepath.Join(parent, "workspaces", "feature")
	if err := os.MkdirAll(mainRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "shared.env"), []byte("SECRET=1"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	Apply(mainRoot, wsPath, []string{"../shared.env"}, nil, &warn)

	if _, err := os.Stat(filepath.Join(parent, "workspaces", "shared.env")); err == nil {
		t.Error("traversal pattern wrote outside wsPath")
	}
	if warn.Len() == 0 {
		t.Error("expected a warning about the rejected traversal pattern")
	}
}

func TestApplyRejectsPathTraversalInSymlink(t *testing.T) {
	parent := t.TempDir()
	mainRoot := filepath.Join(parent, "repo")
	wsPath := filepath.Join(parent, "workspaces", "feature")
	if err := os.MkdirAll(mainRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "shared.env"), []byte("SECRET=1"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	Apply(mainRoot, wsPath, nil, []string{"../shared.env"}, &warn)

	if _, err := os.Lstat(filepath.Join(parent, "workspaces", "shared.env")); err == nil {
		t.Error("traversal pattern wrote outside wsPath")
	}
	if warn.Len() == 0 {
		t.Error("expected a warning about the rejected traversal pattern")
	}
}

func TestApplyPreservesFileSymlinkInsideCopiedDirectory(t *testing.T) {
	mainRoot := t.TempDir()
	wsPath := t.TempDir()

	if err := os.MkdirAll(filepath.Join(mainRoot, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(mainRoot, "secret")
	if err := os.WriteFile(secret, []byte("s3cret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(mainRoot, "config", "local")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	// A sibling regular file must still be copied even though the walk
	// also visits the symlink.
	if err := os.WriteFile(filepath.Join(mainRoot, "config", "other.toml"), []byte("x=1"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	Apply(mainRoot, wsPath, []string{"config"}, nil, &warn)

	dst := filepath.Join(wsPath, "config", "local")
	info, err := os.Lstat(dst)
	if err != nil {
		t.Fatalf("stat copied symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to remain a symlink, got mode %v", dst, info.Mode())
	}
	target, err := os.Readlink(dst)
	if err != nil {
		t.Fatalf("reading copied symlink: %v", err)
	}
	if target != secret {
		t.Errorf("symlink target = %q, want %q", target, secret)
	}
	if _, err := os.Stat(filepath.Join(wsPath, "config", "other.toml")); err != nil {
		t.Errorf("expected sibling file to still be copied: %v", err)
	}
}

func TestApplyPreservesDirSymlinkInsideCopiedDirectory(t *testing.T) {
	mainRoot := t.TempDir()
	wsPath := t.TempDir()

	real := filepath.Join(mainRoot, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(mainRoot, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(mainRoot, "cache", "alias")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	// A file that sorts after "alias" must still get copied: a directory
	// symlink used to abort the whole walk.
	if err := os.WriteFile(filepath.Join(mainRoot, "cache", "zzz.toml"), []byte("z=1"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	Apply(mainRoot, wsPath, []string{"cache"}, nil, &warn)

	dst := filepath.Join(wsPath, "cache", "alias")
	info, err := os.Lstat(dst)
	if err != nil {
		t.Fatalf("stat copied symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to remain a symlink, got mode %v", dst, info.Mode())
	}
	target, err := os.Readlink(dst)
	if err != nil {
		t.Fatalf("reading copied symlink: %v", err)
	}
	if target != real {
		t.Errorf("symlink target = %q, want %q", target, real)
	}
	if _, err := os.Stat(filepath.Join(wsPath, "cache", "zzz.toml")); err != nil {
		t.Errorf("expected the walk to continue past the directory symlink: %v", err)
	}
}

func TestApplySkipsExistingDestinationWithWarning(t *testing.T) {
	mainRoot := t.TempDir()
	wsPath := t.TempDir()

	if err := os.WriteFile(filepath.Join(mainRoot, ".env"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsPath, ".env"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	Apply(mainRoot, wsPath, []string{".env"}, nil, &warn)

	got, err := os.ReadFile(filepath.Join(wsPath, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing" {
		t.Errorf("existing destination was overwritten: got %q", got)
	}
	if warn.Len() == 0 {
		t.Error("expected a warning about the skipped existing destination")
	}
}
