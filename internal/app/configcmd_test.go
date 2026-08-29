package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigShowInRepoWithOverride(t *testing.T) {
	f := newFixture(t)
	cfg := "agent = \"aider\"\n"
	if err := os.WriteFile(filepath.Join(f.mainRoot, ".jumux.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.app.ConfigShow(); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "aider") {
		t.Errorf("expected repo-configured agent in output: %s", out)
	}
	if !strings.Contains(out, "(repo)") {
		t.Errorf("expected agent to be attributed to repo: %s", out)
	}
	if !strings.Contains(out, "(default)") {
		t.Errorf("expected unconfigured keys to be attributed to default: %s", out)
	}
}

func TestConfigShowOutsideRepo(t *testing.T) {
	f := newFixture(t)
	f.app.Getwd = func() (string, error) { return "", os.ErrNotExist }
	if err := f.app.ConfigShow(); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "not in a jj repo") {
		t.Errorf("expected a note about being outside a repo: %s", out)
	}
	if !strings.Contains(out, "claude") {
		t.Errorf("expected default agent value in output: %s", out)
	}
}
