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
