// Package config loads jumux settings from a global TOML file and a
// per-repo .jumux.toml, with per-key repo-over-global precedence.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const RepoFileName = ".jumux.toml"

type Config struct {
	Agent          string `toml:"agent"`
	SelectWindow   *bool  `toml:"select_window"`
	BaseRevision   string `toml:"base_revision"`
	WindowPrefix   string `toml:"window_prefix"`
	SidebarWidth   int    `toml:"sidebar_width"`
	SidebarRefresh int    `toml:"sidebar_refresh"`
	Notify         *bool  `toml:"notify"`
}

func defaults() Config {
	return Config{
		Agent:          "claude",
		BaseRevision:   "trunk()",
		SidebarWidth:   32,
		SidebarRefresh: 2,
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
