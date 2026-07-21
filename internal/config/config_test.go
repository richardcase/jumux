package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent != "claude" || cfg.BaseRevision != "trunk()" || !cfg.SelectWindowEnabled() {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadRepoOverridesGlobalPerKey(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	write(t, global, "agent = \"aider\"\nwindow_prefix = \"g-\"\nselect_window = false\n")
	repoRoot := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repoRoot, RepoFileName), "agent = \"claude --continue\"\n")

	cfg, err := Load(global, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent != "claude --continue" {
		t.Errorf("repo agent should win, got %q", cfg.Agent)
	}
	if cfg.WindowPrefix != "g-" {
		t.Errorf("global window_prefix should survive, got %q", cfg.WindowPrefix)
	}
	if cfg.SelectWindowEnabled() {
		t.Error("global select_window=false should survive")
	}
	if cfg.BaseRevision != "trunk()" {
		t.Errorf("unset base_revision should keep default, got %q", cfg.BaseRevision)
	}
}

func TestLoadMalformed(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.toml")
	write(t, bad, "agent = [broken\n")
	if _, err := Load(bad, ""); err == nil {
		t.Fatal("expected error for malformed TOML")
	}
}

func TestAgentCommandSubstitution(t *testing.T) {
	c := Config{Agent: "claude 'work on {feature}'"}
	if got := c.AgentCommand("auth"); got != "claude 'work on auth'" {
		t.Errorf("got %q", got)
	}
	c = Config{Agent: "claude"}
	if got := c.AgentCommand("auth"); got != "claude" {
		t.Errorf("bare command changed: %q", got)
	}
}
