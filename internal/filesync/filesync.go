// Package filesync copies or symlinks configured paths from a repo's main
// workspace into another workspace directory.
package filesync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Apply resolves copyGlobs and symlinkGlobs (via filepath.Glob, relative to
// mainRoot) and copies/symlinks each match into the same relative path
// under wsPath. A glob matching nothing is skipped silently (the source is
// optional, e.g. a .env file that doesn't exist yet in this repo). A
// destination that already exists is left untouched and reported as a
// warning to warnw, making re-application (`jumux sync`) idempotent.
// Per-entry I/O failures (permission errors, etc.) are also reported as
// warnings rather than returned as errors, since these files are a
// convenience, not essential to the workspace existing.
func Apply(mainRoot, wsPath string, copyGlobs, symlinkGlobs []string, warnw io.Writer) {
	for _, pattern := range copyGlobs {
		applyOne(mainRoot, wsPath, pattern, copyEntry, warnw)
	}
	for _, pattern := range symlinkGlobs {
		applyOne(mainRoot, wsPath, pattern, symlinkEntry, warnw)
	}
}

func applyOne(mainRoot, wsPath, pattern string, do func(src, dst string) error, warnw io.Writer) {
	matches, err := filepath.Glob(filepath.Join(mainRoot, pattern))
	if err != nil {
		_, _ = fmt.Fprintf(warnw, "filesync: invalid pattern %q: %v\n", pattern, err)
		return
	}
	for _, src := range matches {
		rel, err := filepath.Rel(mainRoot, src)
		if err != nil {
			_, _ = fmt.Fprintf(warnw, "filesync: %v\n", err)
			continue
		}
		dst := filepath.Join(wsPath, rel)
		if _, err := os.Lstat(dst); err == nil {
			_, _ = fmt.Fprintf(warnw, "filesync: %s already exists, skipping\n", rel)
			continue
		}
		if err := do(src, dst); err != nil {
			_, _ = fmt.Fprintf(warnw, "filesync: %s: %v\n", rel, err)
		}
	}
}

func copyEntry(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst, info.Mode())
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode())
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}

func symlinkEntry(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Symlink(src, dst)
}
