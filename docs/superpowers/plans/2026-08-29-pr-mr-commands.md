# `jumux pr` / `jumux mr` Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `jumux pr [feature]` and `jumux mr [feature]` commands that push a feature's jj bookmark and open a GitHub PR (`gh pr create`) or GitLab MR (`glab mr create`), prefilled from the jj change description.

**Architecture:** New `internal/jj` wrapper functions for bookmarks/push/description; a new `internal/forge` package holding the shared push+prefill logic; `internal/app` gains a shared feature-inference helper (extracted from `remove.go`) plus `PR`/`MR` methods that call `forge.PreparePush` then shell out to `gh`/`glab`; `main.go` wires up the two subcommands; `jumux doctor` gains non-fatal `gh`/`glab` presence checks.

**Tech Stack:** Go, existing `internal/run.Runner` abstraction (real: `os/exec`, test: `run.FakeRunner`), `jj` and `gh`/`glab` CLIs shelled out to via `Runner`.

**Spec:** `docs/superpowers/specs/2026-08-29-pr-mr-commands-design.md`

## Global Constraints

- Do not add `Co-Authored-By:` trailers or any AI-agent attribution to commit messages (AGENTS.md).
- Keep commit messages concise with an imperative subject line; use conventional-commit-style prefixes (`feat:`, `test:`, `docs:`) where they fit (AGENTS.md).
- Keep all logic in `internal/`; `main.go` only parses arguments and dispatches (AGENTS.md).
- Write table-driven tests alongside the code they test, using `internal/run.FakeRunner` instead of executing real `jj`/`tmux`/`gh`/`glab` (AGENTS.md).
- Re-running `pr`/`mr` after a PR/MR already exists must succeed (push-only), not error (spec).
- No new config fields for remote host detection — `gh`/`glab` handle their own remote/auth (spec, Non-goals).
- Run `make all` (build, test, lint) before considering the change done (AGENTS.md).

---

## Task 1: `internal/jj` bookmark/push/description wrappers

**Files:**
- Modify: `internal/jj/jj.go` (add functions after existing `IsDirty`, following the file's existing style)
- Test: `internal/jj/jj_test.go` (add cases following existing `TestIsDirty`-style tests)

**Interfaces:**
- Consumes: `run.Runner` interface (`internal/run/run.go:14`), `run.FakeRunner` (`internal/run/fake.go`)
- Produces:
  - `func BookmarkSet(r run.Runner, dir, name, rev string) error`
  - `func GitPush(r run.Runner, dir, bookmark string) error`
  - `func Description(r run.Runner, dir, rev string) (string, error)`

- [ ] **Step 1: Write failing tests for all three functions**

Add to `internal/jj/jj_test.go`:

```go
func TestBookmarkSet(t *testing.T) {
	fr := &run.FakeRunner{}
	err := BookmarkSet(fr, "/repo", "myfeature", "myfeature@")
	if err != nil {
		t.Fatalf("BookmarkSet() error = %v", err)
	}
	want := "jj bookmark set myfeature -r myfeature@\n"
	if got := fr.CommandLines(); got != want {
		t.Errorf("CommandLines() = %q, want %q", got, want)
	}
}

func TestBookmarkSetError(t *testing.T) {
	frErr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", errors.New("boom")
	}}
	if err := BookmarkSet(frErr, "/repo", "myfeature", "myfeature@"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGitPush(t *testing.T) {
	fr := &run.FakeRunner{}
	err := GitPush(fr, "/repo", "myfeature")
	if err != nil {
		t.Fatalf("GitPush() error = %v", err)
	}
	want := "jj git push --bookmark myfeature\n"
	if got := fr.CommandLines(); got != want {
		t.Errorf("CommandLines() = %q, want %q", got, want)
	}
}

func TestGitPushError(t *testing.T) {
	frErr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", errors.New("boom")
	}}
	if err := GitPush(frErr, "/repo", "myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDescription(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "Add widget support\n\nThis adds widgets.", nil
	}}
	got, err := Description(fr, "/repo", "myfeature@")
	if err != nil {
		t.Fatalf("Description() error = %v", err)
	}
	want := "Add widget support\n\nThis adds widgets."
	if got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
	wantCmd := "jj log -r myfeature@ --no-graph -T description\n"
	if got := fr.CommandLines(); got != wantCmd {
		t.Errorf("CommandLines() = %q, want %q", got, wantCmd)
	}
}

func TestDescriptionEmpty(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", nil
	}}
	got, err := Description(fr, "/repo", "myfeature@")
	if err != nil {
		t.Fatalf("Description() error = %v", err)
	}
	if got != "" {
		t.Errorf("Description() = %q, want empty", got)
	}
}

func TestDescriptionError(t *testing.T) {
	frErr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", errors.New("boom")
	}}
	if _, err := Description(frErr, "/repo", "myfeature@"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
```

Check the top of `internal/jj/jj_test.go` for an existing `"errors"` import; add it if missing.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/jj/... -run 'TestBookmarkSet|TestGitPush|TestDescription' -v`
Expected: FAIL — `undefined: BookmarkSet` (and similarly for `GitPush`, `Description`)

- [ ] **Step 3: Implement the three functions**

Add to `internal/jj/jj.go` (after `IsDirty`):

```go
// BookmarkSet sets bookmark name to point at rev.
func BookmarkSet(r run.Runner, dir, name, rev string) error {
	_, err := r.Run(dir, "jj", "bookmark", "set", name, "-r", rev)
	return err
}

// GitPush pushes bookmark to its remote.
func GitPush(r run.Runner, dir, bookmark string) error {
	_, err := r.Run(dir, "jj", "git", "push", "--bookmark", bookmark)
	return err
}

// Description returns the full change description for rev.
func Description(r run.Runner, dir, rev string) (string, error) {
	return r.Run(dir, "jj", "log", "-r", rev, "--no-graph", "-T", "description")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/jj/... -v`
Expected: PASS (all tests, including pre-existing ones)

- [ ] **Step 5: Commit**

```bash
git add internal/jj/jj.go internal/jj/jj_test.go
git commit -m "feat: add jj bookmark, push, and description wrappers"
```

---

## Task 2: Extract shared feature-inference helper in `internal/app`

**Files:**
- Modify: `internal/app/remove.go` (move `inferFeature` out, keep call site)
- Modify: `internal/app/app.go` (add the extracted method)
- Test: `internal/app/remove_test.go` (existing tests must still pass unchanged — no new tests required for this task, since behavior is unchanged; confirmed via Step 2/4 below)

**Interfaces:**
- Consumes: `repoContext` type (`internal/app/app.go`), `tmuxctl.CurrentWindowFeature(a.Runner)`, `a.jjWorkspaces`-equivalent lookup already used inside `inferFeature`
- Produces: `func (a *App) inferFeature(ctx *repoContext, names []string) (string, error)` — now a method on `*App` instead of a free function/method private to `remove.go`, callable from `pr.go` and `mr.go` in Task 3

- [ ] **Step 1: Read the current `inferFeature` implementation**

Read `internal/app/remove.go` in full to get its exact current signature and body (it was reported as being defined around lines 101-116 in exploration, but read the live file — do not guess at the body).

- [ ] **Step 2: Run the existing app test suite to record the baseline**

Run: `go test ./internal/app/... -v`
Expected: PASS (baseline before refactor — record this passes before moving code)

- [ ] **Step 3: Move `inferFeature` from `remove.go` to `app.go` unchanged**

Cut the exact function body found in Step 1 out of `internal/app/remove.go` and paste it into `internal/app/app.go`, placing it near `repoContext`/`workspacePath` (the other shared helpers). Do not change its signature, parameter names, or logic — this step is a pure move. If `remove.go`'s call site was `a.inferFeature(ctx, names)`, leave that call site as-is (the method still exists on `*App`, just defined in a different file within the same package — Go doesn't care which file a method is defined in).

- [ ] **Step 4: Run tests to verify nothing broke**

Run: `go test ./internal/app/... -v`
Expected: PASS — identical results to Step 2 (same tests, same pass/fail), confirming the pure move didn't change behavior.

- [ ] **Step 5: Commit**

```bash
git add internal/app/app.go internal/app/remove.go
git commit -m "refactor: move inferFeature to app.go for reuse by pr/mr commands"
```

---

## Task 3: `internal/forge` package — shared push+prefill logic

**Files:**
- Create: `internal/forge/forge.go`
- Create: `internal/forge/forge_test.go`

**Interfaces:**
- Consumes: `run.Runner` (`internal/run/run.go:14`), `jj.BookmarkSet`, `jj.GitPush`, `jj.Description` (Task 1)
- Produces: `func PreparePush(r run.Runner, wsRoot, feature string) (title, body string, err error)` — used by `internal/app/pr.go` and `internal/app/mr.go` in Task 4

- [ ] **Step 1: Write failing tests**

Create `internal/forge/forge_test.go`:

```go
package forge

import (
	"errors"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

func TestPreparePushWithDescription(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 0 && args[0] == "log" {
			return "Add widget support\n\nThis adds widgets.", nil
		}
		return "", nil
	}}

	title, body, err := PreparePush(fr, "/repo/ws", "myfeature")
	if err != nil {
		t.Fatalf("PreparePush() error = %v", err)
	}
	if title != "Add widget support" {
		t.Errorf("title = %q, want %q", title, "Add widget support")
	}
	if body != "This adds widgets." {
		t.Errorf("body = %q, want %q", body, "This adds widgets.")
	}
}

func TestPreparePushEmptyDescriptionFallsBackToFeatureName(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", nil
	}}

	title, body, err := PreparePush(fr, "/repo/ws", "myfeature")
	if err != nil {
		t.Fatalf("PreparePush() error = %v", err)
	}
	if title != "myfeature" {
		t.Errorf("title = %q, want %q", title, "myfeature")
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestPreparePushBookmarkSetError(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 1 && args[0] == "bookmark" {
			return "", errors.New("boom")
		}
		return "", nil
	}}

	if _, _, err := PreparePush(fr, "/repo/ws", "myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPreparePushGitPushError(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 1 && args[0] == "git" {
			return "", errors.New("boom")
		}
		return "", nil
	}}

	if _, _, err := PreparePush(fr, "/repo/ws", "myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPreparePushCommandSequence(t *testing.T) {
	fr := &run.FakeRunner{}
	if _, _, err := PreparePush(fr, "/repo/ws", "myfeature"); err != nil {
		t.Fatalf("PreparePush() error = %v", err)
	}
	want := "jj bookmark set myfeature -r myfeature@\n" +
		"jj git push --bookmark myfeature\n" +
		"jj log -r myfeature@ --no-graph -T description\n"
	if got := fr.CommandLines(); got != want {
		t.Errorf("CommandLines() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/forge/... -v`
Expected: FAIL — package `internal/forge` does not exist yet

- [ ] **Step 3: Implement `PreparePush`**

Create `internal/forge/forge.go`:

```go
// Package forge contains logic shared by jumux's PR and MR commands:
// pushing a feature's jj bookmark and deriving a title/body from its
// change description.
package forge

import (
	"strings"

	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/run"
)

// PreparePush sets feature's bookmark to its working-copy commit,
// pushes it, and derives a title/body from the change description.
// If the description is empty, title falls back to feature and body
// is empty.
func PreparePush(r run.Runner, wsRoot, feature string) (title, body string, err error) {
	rev := feature + "@"

	if err := jj.BookmarkSet(r, wsRoot, feature, rev); err != nil {
		return "", "", err
	}
	if err := jj.GitPush(r, wsRoot, feature); err != nil {
		return "", "", err
	}
	desc, err := jj.Description(r, wsRoot, rev)
	if err != nil {
		return "", "", err
	}

	desc = strings.TrimSpace(desc)
	if desc == "" {
		return feature, "", nil
	}

	lines := strings.SplitN(desc, "\n", 2)
	title = lines[0]
	if len(lines) == 2 {
		body = strings.TrimSpace(lines[1])
	}
	return title, body, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/forge/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/forge/forge.go internal/forge/forge_test.go
git commit -m "feat: add internal/forge package for pr/mr push and prefill"
```

---

## Task 4: `App.PR` and `App.MR` methods

**Files:**
- Create: `internal/app/pr.go`
- Create: `internal/app/pr_test.go`
- Create: `internal/app/mr.go`
- Create: `internal/app/mr_test.go`

**Interfaces:**
- Consumes: `a.repoContext()` (`internal/app/app.go`), `a.inferFeature(ctx, names)` (Task 2), `forge.PreparePush(r, wsRoot, feature)` (Task 3), `a.Runner.Run(dir, name string, args ...string) (string, error)` (`internal/run/run.go`)
- Produces:
  - `func (a *App) PR(feature string) error`
  - `func (a *App) MR(feature string) error`
  Both consumed by `main.go` in Task 5.

- [ ] **Step 1: Read `remove.go` and `app.go` for the exact bootstrap pattern**

Read `internal/app/remove.go` and the `repoContext`/`workspacePath`/`inferFeature` definitions in `internal/app/app.go` (after Task 2's move) to match field names and error-wrapping style exactly (e.g. whether `ctx.WsRoot` or another field name is used, and how `jj.Workspaces` is called to build the `names []string` argument to `inferFeature`).

- [ ] **Step 2: Write failing tests for `PR`**

Create `internal/app/pr_test.go`, modeled on the existing `remove_test.go` fixture (`f := fixture(t)` or equivalent — match whatever helper `internal/app/app_test.go` actually defines):

```go
package app

import (
	"strings"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

func TestPRExplicitFeature(t *testing.T) {
	f := fixture(t)
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 0 && args[0] == "log" {
			return "Add widget support\n\nThis adds widgets.", nil
		}
		return "", nil
	}

	if err := f.app.PR("myfeature"); err != nil {
		t.Fatalf("PR() error = %v", err)
	}

	found := false
	for _, c := range f.runner.Calls {
		if c.Name == "gh" && len(c.Args) > 0 && c.Args[0] == "pr" {
			found = true
			wantArgs := []string{"pr", "create", "--title", "Add widget support", "--body", "This adds widgets."}
			if strings.Join(c.Args, " ") != strings.Join(wantArgs, " ") {
				t.Errorf("gh args = %v, want %v", c.Args, wantArgs)
			}
		}
	}
	if !found {
		t.Error("expected a gh pr create call, got none")
	}
}

func TestPRAlreadyExistsSucceeds(t *testing.T) {
	f := fixture(t)
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "gh" {
			return "", errors.New("a pull request for branch \"myfeature\" into branch \"main\" already exists")
		}
		return "", nil
	}

	if err := f.app.PR("myfeature"); err != nil {
		t.Fatalf("PR() error = %v, want nil (already-exists should succeed)", err)
	}
}

func TestPRGhFailureIsError(t *testing.T) {
	f := fixture(t)
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "gh" {
			return "", errors.New("gh: command not found")
		}
		return "", nil
	}

	if err := f.app.PR("myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
```

Add `"errors"` to the imports if used and not already present. If `fixture(t)` returns a different field name than `f.runner`/`f.app` (e.g. `f.r`, `f.a`), adjust to match — check `internal/app/app_test.go` and `internal/app/remove_test.go` for the exact fixture API before finalizing this step; do not guess blindly, read the file.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/app/... -run TestPR -v`
Expected: FAIL — `undefined: (*App).PR` (or compile error referencing missing `pr.go`)

- [ ] **Step 4: Implement `App.PR`**

Create `internal/app/pr.go`:

```go
package app

import (
	"fmt"
	"strings"

	"github.com/richardcase/jumux/internal/forge"
	"github.com/richardcase/jumux/internal/jj"
)

// PR pushes feature's bookmark and opens a GitHub pull request for it.
// If feature is empty, it is inferred from the current workspace/window.
func (a *App) PR(feature string) error {
	ctx, err := a.repoContext()
	if err != nil {
		return err
	}

	if feature == "" {
		names, err := jj.Workspaces(a.Runner, ctx.MainRoot)
		if err != nil {
			return err
		}
		feature, err = a.inferFeature(ctx, names)
		if err != nil {
			return err
		}
	}

	title, body, err := forge.PreparePush(a.Runner, ctx.WsRoot, feature)
	if err != nil {
		return err
	}

	out, err := a.Runner.Run(ctx.WsRoot, "gh", "pr", "create", "--title", title, "--body", body)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			fmt.Fprintf(a.Out, "pushed %s; a pull request already exists\n", feature)
			return nil
		}
		return err
	}
	fmt.Fprintln(a.Out, out)
	return nil
}
```

Adjust `ctx.MainRoot`/`ctx.WsRoot` field names and `jj.Workspaces` call signature to whatever Step 1's reading found if they differ from this draft — this code must match the real `repoContext` struct fields exactly.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/app/... -run TestPR -v`
Expected: PASS

- [ ] **Step 6: Write failing tests for `MR`**

Create `internal/app/mr_test.go`, mirroring `pr_test.go` exactly but asserting `glab mr create --title ... --description ...`:

```go
package app

import (
	"errors"
	"strings"
	"testing"
)

func TestMRExplicitFeature(t *testing.T) {
	f := fixture(t)
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "jj" && len(args) > 0 && args[0] == "log" {
			return "Add widget support\n\nThis adds widgets.", nil
		}
		return "", nil
	}

	if err := f.app.MR("myfeature"); err != nil {
		t.Fatalf("MR() error = %v", err)
	}

	found := false
	for _, c := range f.runner.Calls {
		if c.Name == "glab" && len(c.Args) > 0 && c.Args[0] == "mr" {
			found = true
			wantArgs := []string{"mr", "create", "--title", "Add widget support", "--description", "This adds widgets."}
			if strings.Join(c.Args, " ") != strings.Join(wantArgs, " ") {
				t.Errorf("glab args = %v, want %v", c.Args, wantArgs)
			}
		}
	}
	if !found {
		t.Error("expected a glab mr create call, got none")
	}
}

func TestMRAlreadyExistsSucceeds(t *testing.T) {
	f := fixture(t)
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "glab" {
			return "", errors.New("merge request already exists")
		}
		return "", nil
	}

	if err := f.app.MR("myfeature"); err != nil {
		t.Fatalf("MR() error = %v, want nil (already-exists should succeed)", err)
	}
}

func TestMRGlabFailureIsError(t *testing.T) {
	f := fixture(t)
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "glab" {
			return "", errors.New("glab: command not found")
		}
		return "", nil
	}

	if err := f.app.MR("myfeature"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
```

- [ ] **Step 7: Run tests to verify they fail**

Run: `go test ./internal/app/... -run TestMR -v`
Expected: FAIL — `undefined: (*App).MR`

- [ ] **Step 8: Implement `App.MR`**

Create `internal/app/mr.go`, structurally identical to `pr.go` but targeting `glab`:

```go
package app

import (
	"fmt"
	"strings"

	"github.com/richardcase/jumux/internal/forge"
	"github.com/richardcase/jumux/internal/jj"
)

// MR pushes feature's bookmark and opens a GitLab merge request for it.
// If feature is empty, it is inferred from the current workspace/window.
func (a *App) MR(feature string) error {
	ctx, err := a.repoContext()
	if err != nil {
		return err
	}

	if feature == "" {
		names, err := jj.Workspaces(a.Runner, ctx.MainRoot)
		if err != nil {
			return err
		}
		feature, err = a.inferFeature(ctx, names)
		if err != nil {
			return err
		}
	}

	title, body, err := forge.PreparePush(a.Runner, ctx.WsRoot, feature)
	if err != nil {
		return err
	}

	out, err := a.Runner.Run(ctx.WsRoot, "glab", "mr", "create", "--title", title, "--description", body)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			fmt.Fprintf(a.Out, "pushed %s; a merge request already exists\n", feature)
			return nil
		}
		return err
	}
	fmt.Fprintln(a.Out, out)
	return nil
}
```

- [ ] **Step 9: Run all app tests to verify everything passes**

Run: `go test ./internal/app/... -v`
Expected: PASS (all tests, including pre-existing ones)

- [ ] **Step 10: Commit**

```bash
git add internal/app/pr.go internal/app/pr_test.go internal/app/mr.go internal/app/mr_test.go
git commit -m "feat: add App.PR and App.MR methods"
```

---

## Task 5: Wire up `jumux pr` / `jumux mr` in `main.go`

**Files:**
- Modify: `main.go` (add two `case` blocks to the dispatch switch, update `usage` const)

**Interfaces:**
- Consumes: `a.PR(feature string) error`, `a.MR(feature string) error` (Task 4)
- Produces: CLI entry points `jumux pr [feature]`, `jumux mr [feature]`

- [ ] **Step 1: Read `main.go`'s `remove` case as the template**

Read `main.go` in full, focusing on the `case "remove":` block (it takes an optional positional name plus a `-f` flag) and the `usage` const, to match formatting exactly.

- [ ] **Step 2: Add `usage` entries**

Edit the `usage` const to add lines for `pr` and `mr` alongside the existing command list, matching its existing formatting style, e.g.:

```
  jumux pr [feature]      push feature's bookmark and open a GitHub PR
  jumux mr [feature]      push feature's bookmark and open a GitLab MR
```//exact indentation and column alignment must match the surrounding lines in the real file — copy the existing style precisely rather than this sketch.

- [ ] **Step 3: Add `case "pr":` and `case "mr":` to the dispatch switch**

```go
	case "pr":
		fs := flag.NewFlagSet("pr", flag.ExitOnError)
		fs.Parse(os.Args[2:])
		if fs.NArg() > 1 {
			fmt.Fprintln(os.Stderr, "usage: jumux pr [feature]")
			os.Exit(2)
		}
		feature := ""
		if fs.NArg() == 1 {
			feature = fs.Arg(0)
		}
		err = a.PR(feature)
	case "mr":
		fs := flag.NewFlagSet("mr", flag.ExitOnError)
		fs.Parse(os.Args[2:])
		if fs.NArg() > 1 {
			fmt.Fprintln(os.Stderr, "usage: jumux mr [feature]")
			os.Exit(2)
		}
		feature := ""
		if fs.NArg() == 1 {
			feature = fs.Arg(0)
		}
		err = a.MR(feature)
```

Insert these in alphabetical/logical order matching the existing switch's ordering convention (check where `remove`/`list` sit relative to each other in the real file and place `pr`/`mr` consistently).

- [ ] **Step 4: Build and smoke-test the CLI wiring**

Run: `go build -o bin/jumux . && ./bin/jumux 2>&1 | head -20`
Expected: build succeeds; usage output includes the new `pr`/`mr` lines.

Run: `./bin/jumux pr a b 2>&1; echo "exit=$?"`
Expected: prints `usage: jumux pr [feature]` and exits with code 2 (too many args).

- [ ] **Step 5: Run the full test suite**

Run: `go test ./... -v`
Expected: PASS across all packages

- [ ] **Step 6: Commit**

```bash
git add main.go
git commit -m "feat: wire up jumux pr and jumux mr commands"
```

---

## Task 6: `jumux doctor` checks for `gh`/`glab`

**Files:**
- Modify: `internal/app/doctor.go`
- Test: `internal/app/doctor_test.go`

**Interfaces:**
- Consumes: whatever existing pattern `doctor.go` uses to check for `jj`/`tmux` presence (read the file first — likely `a.Runner.Run(dir, "gh", "--version")` or similar, or `exec.LookPath` via an injected lookup)
- Produces: two additional non-fatal doctor report lines for `gh` and `glab`

- [ ] **Step 1: Read `internal/app/doctor.go` and its test file in full**

This file wasn't covered by prior exploration — read it completely before writing anything, to find the exact pattern used for existing checks (e.g. how a check's pass/fail/warn state is represented and printed, and whether checks use `a.Runner` or `exec.LookPath` directly).

- [ ] **Step 2: Write failing tests for the two new checks**

Add table-driven cases to `internal/app/doctor_test.go` matching the existing test style found in Step 1 — one asserting the "found" output when the fake runner's handler succeeds for `gh --version` / `glab --version` (or whatever probe command matches the real implementation), and one asserting the "not found" output when the handler returns an error for those commands, in both cases asserting doctor's overall success/failure status is unaffected (still passes doctor even when `gh`/`glab` are missing).

Do not write exact test code here — the assertion style depends entirely on what Step 1 reveals about `doctor.go`'s existing output format (e.g. a `[]CheckResult` slice vs. printed strings). Match that exactly.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/app/... -run TestDoctor -v`
Expected: FAIL — missing `gh`/`glab` checks in output

- [ ] **Step 4: Implement the two checks**

Add `gh` and `glab` checks to `doctor.go` following the exact structure of the existing `jj`/`tmux` checks found in Step 1, but marked non-fatal (informational) so a missing binary doesn't fail doctor overall — mirror however the existing code marks a check as advisory vs. required, if such a distinction already exists; if it doesn't, add the minimal distinction needed (e.g. a boolean field on the check struct) rather than inventing a parallel mechanism.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/app/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/app/doctor.go internal/app/doctor_test.go
git commit -m "feat: add gh/glab presence checks to jumux doctor"
```

---

## Task 7: Full verification pass

**Files:** none (verification only)

- [ ] **Step 1: Run `make all`**

Run: `make all`
Expected: build, test, and lint all pass with no errors

- [ ] **Step 2: Fix any lint/vet issues surfaced**

If `make all` reports issues, fix them in the relevant files from Tasks 1-6 and re-run `make all` until clean. Do not use `--no-verify` or disable lint rules to work around failures.

- [ ] **Step 3: Manual smoke test in a scratch jj+tmux repo (if available)**

If a local jj repo with a GitHub or GitLab remote is available for manual testing: run `jumux add <feature>`, make a small change inside the feature workspace, then run `jumux pr` (or `jumux mr`) from within that feature's tmux window with no arguments, and confirm:
- the bookmark is created and pushed,
- `gh pr create` (or `glab mr create`) runs with the change's description as title/body,
- re-running the same command succeeds without error.

If no such repo is available, state explicitly that this manual step was skipped and why, rather than claiming it was verified.

- [ ] **Step 4: Final commit if any fixes were made in Step 2**

```bash
git add -A
git commit -m "fix: address lint/vet issues from pr/mr implementation"
```

(Skip this step if Step 2 required no changes.)
