// Package jj wraps the jujutsu CLI operations jumux needs.
package jj

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/richardcase/jumux/internal/run"
)

// Root returns the root of the workspace containing dir.
func Root(r run.Runner, dir string) (string, error) {
	return r.Run(dir, "jj", "root", "--ignore-working-copy")
}

// MainRoot resolves the main (default) workspace root given any workspace
// root. In a secondary workspace .jj/repo is a plain file containing the
// path to the real repo store; in the main workspace it is a directory.
func MainRoot(workspaceRoot string) (string, error) {
	repoPath := filepath.Join(workspaceRoot, ".jj", "repo")
	fi, err := os.Stat(repoPath)
	if err != nil {
		return "", fmt.Errorf("not a jj workspace root: %s: %w", workspaceRoot, err)
	}
	if fi.IsDir() {
		return workspaceRoot, nil
	}
	contents, err := os.ReadFile(repoPath)
	if err != nil {
		return "", fmt.Errorf("reading workspace repo pointer: %w", err)
	}
	// Contents point at <mainRoot>/.jj/repo, absolute or relative to the
	// pointer file's own directory (<workspaceRoot>/.jj).
	storePath := strings.TrimSpace(string(contents))
	if !filepath.IsAbs(storePath) {
		storePath = filepath.Join(workspaceRoot, ".jj", storePath)
	}
	return filepath.Clean(filepath.Dir(filepath.Dir(storePath))), nil
}

// Installed reports whether the jj binary is available and runnable.
func Installed(r run.Runner) error {
	_, err := r.Run("", "jj", "--version")
	return err
}

// ResolveRevision reports whether rev resolves to a revision in dir. Unlike
// WorkspaceAdd it is non-mutating, so it is safe to use as a pure check.
func ResolveRevision(r run.Runner, dir, rev string) error {
	_, err := r.Run(dir, "jj", "log", "-r", rev, "--no-graph", "-T", `""`)
	return err
}

// IsColocated reports whether root has both .jj and .git.
func IsColocated(root string) bool {
	if _, err := os.Stat(filepath.Join(root, ".jj")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return false
	}
	return true
}

// Workspaces returns the workspace names from `jj workspace list`
// (including "default").
func Workspaces(r run.Runner, mainRoot string) ([]string, error) {
	out, err := r.Run(mainRoot, "jj", "workspace", "list", "--ignore-working-copy")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		name, _, ok := strings.Cut(line, ":")
		if ok && name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// WorkspaceAdd creates a workspace named name at path, based on rev.
func WorkspaceAdd(r run.Runner, mainRoot, name, path, rev string) error {
	_, err := r.Run(mainRoot, "jj", "workspace", "add", "--name", name, "-r", rev, path)
	return err
}

// WorkspaceForget removes the workspace from the repo (never deletes commits).
func WorkspaceForget(r run.Runner, mainRoot, name string) error {
	_, err := r.Run(mainRoot, "jj", "workspace", "forget", name)
	return err
}

// IsDirty reports whether the workspace's working-copy commit is non-empty.
// It runs jj inside wsPath so the working copy is snapshotted first,
// giving an accurate answer.
func IsDirty(r run.Runner, wsPath, name string) (bool, error) {
	out, err := r.Run(wsPath, "jj", "log", "-r", name+"@", "--no-graph",
		"-T", `if(empty, "clean", "dirty")`)
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "dirty"), nil
}
