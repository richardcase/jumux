package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShowAllDefaults(t *testing.T) {
	_, fields, err := Show("", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fields {
		if f.Source != SourceDefault {
			t.Errorf("key %q: source = %q, want default", f.Key, f.Source)
		}
	}
}

func TestShowReportsGlobalAndRepoSources(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	write(t, global, "agent = \"aider\"\nnotify = false\n")
	repoRoot := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repoRoot, RepoFileName), "base_revision = \"main\"\n")

	cfg, fields, err := Show(global, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent != "aider" || cfg.BaseRevision != "main" || cfg.NotifyEnabled() {
		t.Fatalf("unexpected merged config: %+v", cfg)
	}

	byKey := map[string]Field{}
	for _, f := range fields {
		byKey[f.Key] = f
	}
	if got := byKey["agent"]; got.Source != SourceGlobal || got.Value != "aider" {
		t.Errorf("agent field = %+v", got)
	}
	if got := byKey["notify"]; got.Source != SourceGlobal || got.Value != "false" {
		t.Errorf("notify field = %+v", got)
	}
	if got := byKey["base_revision"]; got.Source != SourceRepo || got.Value != "main" {
		t.Errorf("base_revision field = %+v", got)
	}
	if got := byKey["window_prefix"]; got.Source != SourceDefault {
		t.Errorf("window_prefix field = %+v, want default", got)
	}
}

func TestFormatFields(t *testing.T) {
	var buf bytes.Buffer
	FormatFields(&buf, []Field{{Key: "agent", Value: "claude", Source: SourceDefault}})
	out := buf.String()
	if !strings.Contains(out, "agent") || !strings.Contains(out, "claude") || !strings.Contains(out, "default") {
		t.Errorf("unexpected output: %q", out)
	}
}
