// Package config loads agentmux settings from a global TOML file and a
// per-repo .agentmux.toml, with per-key repo-over-global precedence.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const RepoFileName = ".agentmux.toml"

type Config struct {
	Agent        string `toml:"agent"`
	SelectWindow *bool  `toml:"select_window"`
	BaseRevision string `toml:"base_revision"`
	WindowPrefix string `toml:"window_prefix"`
}

func defaults() Config {
	return Config{
		Agent:        "claude",
		BaseRevision: "trunk()",
	}
}

// GlobalPath returns the default global config location
// (~/.config/agentmux/config.toml on Linux).
func GlobalPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "agentmux", "config.toml")
}

// Load merges defaults <- globalPath <- <repoRoot>/.agentmux.toml.
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

// AgentCommand returns the agent command with any {feature} placeholder
// substituted.
func (c Config) AgentCommand(feature string) string {
	return strings.ReplaceAll(c.Agent, "{feature}", feature)
}

// SelectWindowEnabled reports whether add should switch to the new window
// (default true).
func (c Config) SelectWindowEnabled() bool {
	return c.SelectWindow == nil || *c.SelectWindow
}
