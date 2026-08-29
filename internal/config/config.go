// Package config loads jumux settings from a global TOML file and a
// per-repo .jumux.toml, with per-key repo-over-global precedence.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const RepoFileName = ".jumux.toml"

// DefaultBaseRevision is the built-in base_revision value. jj's trunk()
// revset alias resolves via remote bookmarks, so it can fail to resolve in
// local-only repos with no remote configured.
const DefaultBaseRevision = "trunk()"

type Config struct {
	Agent           string `toml:"agent"`
	SelectWindow    *bool  `toml:"select_window"`
	BaseRevision    string `toml:"base_revision"`
	WindowPrefix    string `toml:"window_prefix"`
	SidebarWidth    int    `toml:"sidebar_width"`
	SidebarRefresh  int    `toml:"sidebar_refresh"`
	Notify          *bool  `toml:"notify"`
	StaleAfterHours int    `toml:"stale_after_hours"`
	// NotifyQuietStart and NotifyQuietEnd bound a daily "HH:MM"-"HH:MM"
	// (24h, local time) window during which status-change notifications
	// are suppressed. A window that wraps past midnight (start > end) is
	// supported. Leave both unset to disable quiet hours.
	NotifyQuietStart string `toml:"notify_quiet_start"`
	NotifyQuietEnd   string `toml:"notify_quiet_end"`
	// NotifyWebhook, if set, is a URL that notifications are POSTed to
	// (as JSON) in addition to the OS desktop notification.
	NotifyWebhook string `toml:"notify_webhook"`
	// Templates are named presets bundling base_revision/agent/window
	// options for a recurring kind of feature, selected via
	// `jumux add --template <name>`. A template defined in the repo file
	// fully replaces a global template of the same name (fields are not
	// merged individually across files).
	Templates map[string]Template `toml:"templates"`
}

// Template bundles overrides for a named preset combination of
// base_revision, agent, and window options.
type Template struct {
	Agent        string `toml:"agent"`
	BaseRevision string `toml:"base_revision"`
	SelectWindow *bool  `toml:"select_window"`
	WindowPrefix string `toml:"window_prefix"`
}

func defaults() Config {
	return Config{
		Agent:           "claude",
		BaseRevision:    DefaultBaseRevision,
		SidebarWidth:    32,
		SidebarRefresh:  2,
		StaleAfterHours: 168, // 7 days
	}
}

// GlobalPath returns the default global config location
// (~/.config/jumux/config.toml on Linux).
func GlobalPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "jumux", "config.toml")
}

// Load merges defaults <- globalPath <- <repoRoot>/.jumux.toml.
// Missing files are ignored; malformed TOML is an error naming the file.
// Decoding successive files into the same struct leaves absent keys
// untouched, giving per-key override semantics.
func Load(globalPath, repoRoot string) (Config, error) {
	cfg := defaults()
	paths := []string{globalPath}
	if repoRoot != "" {
		paths = append(paths, filepath.Join(repoRoot, RepoFileName))
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if _, err := toml.DecodeFile(p, &cfg); err != nil {
			return Config{}, fmt.Errorf("parsing %s: %w", p, err)
		}
	}
	return cfg, nil
}

// AgentCommand returns the agent command to launch for feature, with any
// {feature} placeholder substituted. If override is non-empty it is used
// in place of the configured agent.
func (c Config) AgentCommand(feature, override string) string {
	cmd := c.Agent
	if override != "" {
		cmd = override
	}
	return strings.ReplaceAll(cmd, "{feature}", feature)
}

// SelectWindowEnabled reports whether add should switch to the new window
// (default true).
func (c Config) SelectWindowEnabled() bool {
	return c.SelectWindow == nil || *c.SelectWindow
}

// NotifyEnabled reports whether hook status changes should send an OS
// desktop notification (default true).
func (c Config) NotifyEnabled() bool {
	return c.Notify == nil || *c.Notify
}

// WithTemplate returns a copy of c with the named template's non-zero
// fields applied over the base config (agent, base_revision,
// select_window, window_prefix). An empty name is a no-op and returns c
// unchanged. It errors if name is non-empty but no such template exists.
func (c Config) WithTemplate(name string) (Config, error) {
	if name == "" {
		return c, nil
	}
	t, ok := c.Templates[name]
	if !ok {
		return Config{}, fmt.Errorf("unknown template %q", name)
	}
	out := c
	if t.Agent != "" {
		out.Agent = t.Agent
	}
	if t.BaseRevision != "" {
		out.BaseRevision = t.BaseRevision
	}
	if t.SelectWindow != nil {
		out.SelectWindow = t.SelectWindow
	}
	if t.WindowPrefix != "" {
		out.WindowPrefix = t.WindowPrefix
	}
	return out, nil
}

// InQuietHours reports whether t's local time-of-day falls within the
// configured notify_quiet_start/notify_quiet_end window. It returns false
// (quiet hours disabled) if either bound is unset or unparsable, or if
// they're equal (a zero-length window).
func (c Config) InQuietHours(t time.Time) bool {
	start, ok1 := parseClock(c.NotifyQuietStart)
	end, ok2 := parseClock(c.NotifyQuietEnd)
	if !ok1 || !ok2 || start == end {
		return false
	}
	cur := t.Hour()*60 + t.Minute()
	if start < end {
		return cur >= start && cur < end
	}
	// The window wraps past midnight, e.g. 22:00-06:00.
	return cur >= start || cur < end
}

// parseClock parses "HH:MM" (24h) into minutes since midnight.
func parseClock(s string) (int, bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	h, errH := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// SidebarWidthCols returns the sidebar pane width in columns (default 32).
func (c Config) SidebarWidthCols() int {
	if c.SidebarWidth <= 0 {
		return 32
	}
	return c.SidebarWidth
}

// SidebarRefreshInterval returns the sidebar refresh interval (default 2s).
func (c Config) SidebarRefreshInterval() time.Duration {
	if c.SidebarRefresh <= 0 {
		return 2 * time.Second
	}
	return time.Duration(c.SidebarRefresh) * time.Second
}

// StaleThreshold returns how long a feature may sit idle (no jj changes and
// no hook status updates) before it is flagged stale, and whether stale
// detection is enabled at all. Setting stale_after_hours to 0 or below
// disables it.
func (c Config) StaleThreshold() (time.Duration, bool) {
	if c.StaleAfterHours <= 0 {
		return 0, false
	}
	return time.Duration(c.StaleAfterHours) * time.Hour, true
}
