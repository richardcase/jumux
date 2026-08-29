package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
	if !cfg.NotifyEnabled() {
		t.Error("notify should default to enabled")
	}
	if cfg.SidebarWidthCols() != 32 || cfg.SidebarRefreshInterval() != 2*time.Second {
		t.Errorf("unexpected sidebar defaults: %+v", cfg)
	}
	if d, ok := cfg.StaleThreshold(); !ok || d != 168*time.Hour {
		t.Errorf("unexpected stale default: %v, %v", d, ok)
	}
}

func TestStaleThreshold(t *testing.T) {
	tests := []struct {
		name  string
		hours int
		want  time.Duration
		ok    bool
	}{
		{name: "default disabled by zero", hours: 0, want: 0, ok: false},
		{name: "negative disables", hours: -1, want: 0, ok: false},
		{name: "positive enables", hours: 48, want: 48 * time.Hour, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{StaleAfterHours: tt.hours}
			d, ok := c.StaleThreshold()
			if d != tt.want || ok != tt.ok {
				t.Errorf("StaleThreshold() = %v, %v, want %v, %v", d, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSidebarOverrides(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	write(t, global, "sidebar_width = 40\n")
	repoRoot := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repoRoot, RepoFileName), "sidebar_refresh = 5\n")

	cfg, err := Load(global, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SidebarWidthCols() != 40 {
		t.Errorf("global sidebar_width should apply, got %d", cfg.SidebarWidthCols())
	}
	if cfg.SidebarRefreshInterval() != 5*time.Second {
		t.Errorf("repo sidebar_refresh should apply, got %v", cfg.SidebarRefreshInterval())
	}
}

func TestSidebarAccessorClamping(t *testing.T) {
	for _, c := range []Config{{}, {SidebarWidth: -3, SidebarRefresh: -1}} {
		if c.SidebarWidthCols() != 32 {
			t.Errorf("width %d should clamp to 32, got %d", c.SidebarWidth, c.SidebarWidthCols())
		}
		if c.SidebarRefreshInterval() != 2*time.Second {
			t.Errorf("refresh %d should clamp to 2s, got %v", c.SidebarRefresh, c.SidebarRefreshInterval())
		}
	}
}

func TestLoadRepoOverridesGlobalPerKey(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	write(t, global, "agent = \"aider\"\nwindow_prefix = \"g-\"\nselect_window = false\nnotify = false\n")
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
	if cfg.NotifyEnabled() {
		t.Error("global notify=false should survive")
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
	if got := c.AgentCommand("auth", ""); got != "claude 'work on auth'" {
		t.Errorf("got %q", got)
	}
	c = Config{Agent: "claude"}
	if got := c.AgentCommand("auth", ""); got != "claude" {
		t.Errorf("bare command changed: %q", got)
	}
}

func TestWithTemplateAppliesOverrides(t *testing.T) {
	falseVal := false
	c := Config{
		Agent:        "claude",
		BaseRevision: "trunk()",
		WindowPrefix: "",
		Templates: map[string]Template{
			"bugfix": {
				Agent:        "claude 'fix {feature}'",
				BaseRevision: "main",
				SelectWindow: &falseVal,
				WindowPrefix: "bug-",
			},
		},
	}
	out, err := c.WithTemplate("bugfix")
	if err != nil {
		t.Fatal(err)
	}
	if out.Agent != "claude 'fix {feature}'" || out.BaseRevision != "main" || out.WindowPrefix != "bug-" {
		t.Errorf("unexpected result: %+v", out)
	}
	if out.SelectWindowEnabled() {
		t.Error("template's select_window=false should apply")
	}
}

func TestWithTemplateEmptyNameIsNoOp(t *testing.T) {
	c := Config{Agent: "claude"}
	out, err := c.WithTemplate("")
	if err != nil {
		t.Fatal(err)
	}
	if out.Agent != "claude" {
		t.Errorf("unexpected result: %+v", out)
	}
}

func TestWithTemplateUnknownErrors(t *testing.T) {
	c := Config{}
	if _, err := c.WithTemplate("nope"); err == nil {
		t.Fatal("expected an error for an unknown template")
	}
}

func TestWithTemplateLeavesUnsetFieldsAlone(t *testing.T) {
	c := Config{
		Agent:        "claude",
		BaseRevision: "trunk()",
		WindowPrefix: "g-",
		Templates: map[string]Template{
			"bugfix": {BaseRevision: "main"},
		},
	}
	out, err := c.WithTemplate("bugfix")
	if err != nil {
		t.Fatal(err)
	}
	if out.Agent != "claude" || out.WindowPrefix != "g-" {
		t.Errorf("unset template fields should not override base config: %+v", out)
	}
	if out.BaseRevision != "main" {
		t.Errorf("template base_revision should apply, got %q", out.BaseRevision)
	}
}

func TestLoadParsesTemplates(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	write(t, global, "[templates.bugfix]\nagent = \"claude fix\"\nbase_revision = \"main\"\n")
	cfg, err := Load(global, "")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, ok := cfg.Templates["bugfix"]
	if !ok {
		t.Fatal("expected bugfix template to be loaded")
	}
	if tmpl.Agent != "claude fix" || tmpl.BaseRevision != "main" {
		t.Errorf("unexpected template: %+v", tmpl)
	}
}

func TestAgentCommandOverride(t *testing.T) {
	c := Config{Agent: "claude"}
	if got := c.AgentCommand("auth", "aider 'work on {feature}'"); got != "aider 'work on auth'" {
		t.Errorf("override should win and substitute {feature}, got %q", got)
	}
	if got := c.AgentCommand("auth", ""); got != "claude" {
		t.Errorf("empty override should fall back to configured agent, got %q", got)
	}
}
