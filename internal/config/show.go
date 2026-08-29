package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"text/tabwriter"

	"github.com/BurntSushi/toml"
)

// Source names where an effective config value's file it was sourced from.
type Source string

const (
	// SourceDefault means the key was left at its built-in default: no
	// loaded file set it.
	SourceDefault Source = "default"
	// SourceGlobal means the global config file set the effective value.
	SourceGlobal Source = "global"
	// SourceRepo means the per-repo config file set the effective value.
	SourceRepo Source = "repo"
)

// Field is one effective config key: its value and which file it came
// from.
type Field struct {
	Key    string
	Value  string
	Source Source
}

// Show loads the merged config exactly like Load, additionally reporting,
// for each key, whether its effective value came from the repo file, the
// global file, or the built-in default. globalPath and repoRoot are used
// the same way as in Load.
func Show(globalPath, repoRoot string) (Config, []Field, error) {
	cfg := defaults()

	type file struct {
		path   string
		source Source
	}
	files := []file{{globalPath, SourceGlobal}}
	if repoRoot != "" {
		files = append(files, file{filepath.Join(repoRoot, RepoFileName), SourceRepo})
	}

	definedIn := map[string]Source{}
	for _, f := range files {
		if f.path == "" {
			continue
		}
		if _, err := os.Stat(f.path); err != nil {
			continue
		}
		meta, err := toml.DecodeFile(f.path, &cfg)
		if err != nil {
			return Config{}, nil, fmt.Errorf("parsing %s: %w", f.path, err)
		}
		for _, k := range meta.Keys() {
			// Only track top-level keys; nested keys (e.g. inside
			// [templates.x]) report under their own top-level entry.
			if len(k) > 0 {
				definedIn[k[0]] = f.source
			}
		}
	}

	source := func(key string) Source {
		if s, ok := definedIn[key]; ok {
			return s
		}
		return SourceDefault
	}

	fields := []Field{
		{"agent", cfg.Agent, source("agent")},
		{"select_window", strconv.FormatBool(cfg.SelectWindowEnabled()), source("select_window")},
		{"base_revision", cfg.BaseRevision, source("base_revision")},
		{"window_prefix", cfg.WindowPrefix, source("window_prefix")},
		{"sidebar_width", strconv.Itoa(cfg.SidebarWidthCols()), source("sidebar_width")},
		{"sidebar_refresh", cfg.SidebarRefreshInterval().String(), source("sidebar_refresh")},
		{"notify", strconv.FormatBool(cfg.NotifyEnabled()), source("notify")},
	}
	return cfg, fields, nil
}

// FormatFields writes fields as an aligned key/value/source table.
func FormatFields(w io.Writer, fields []Field) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, f := range fields {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t(%s)\n", f.Key, f.Value, f.Source)
	}
	_ = tw.Flush()
}
