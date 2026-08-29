package app

import (
	"fmt"
	"strings"

	"github.com/richardcase/jumux/internal/forge"
	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

// doctorCheck is the result of one environment check.
type doctorCheck struct {
	Name        string
	OK          bool
	Detail      string
	Remediation string
	// Advisory marks a check as informational: a failing advisory check is
	// reported as a warning but does not fail doctor overall.
	Advisory bool
}

// Doctor runs a set of preflight checks (jj installed, repo colocated,
// tmux running, base_revision resolves, Claude Code hooks installed, gh/glab
// installed) and prints a pass/fail/warn checklist with remediation for each
// failure. Checks run independently of one another so a single run gives the
// fullest diagnostic picture. The gh and glab checks are advisory: a repo
// typically only needs one of the two, so a missing binary is reported as a
// warning and does not fail doctor overall. It returns an error if any
// non-advisory check failed.
func (a *App) Doctor() error {
	var checks []doctorCheck
	checks = append(checks, a.checkJJInstalled())

	ctx, ctxErr := a.repoContext()
	if ctxErr != nil {
		checks = append(checks,
			doctorCheck{
				Name:        "repo colocated with git",
				Detail:      ctxErr.Error(),
				Remediation: "run jumux from inside a jj repo",
			},
			doctorCheck{
				Name:        "base_revision resolves",
				Detail:      "skipped: " + ctxErr.Error(),
				Remediation: "run jumux from inside a jj repo",
			},
		)
	} else {
		checks = append(checks, checkColocated(ctx.MainRoot), a.checkBaseRevision(ctx))
	}

	checks = append(checks, a.checkTmux(), a.checkClaudeHooks(), a.checkGH(), a.checkGlab())

	allOK := true
	for _, c := range checks {
		status := "PASS"
		if !c.OK {
			if c.Advisory {
				status = "WARN"
			} else {
				status = "FAIL"
				allOK = false
			}
		}
		line := fmt.Sprintf("[%s] %s", status, c.Name)
		if c.Detail != "" {
			line += " — " + c.Detail
		}
		_, _ = fmt.Fprintln(a.Out, line)
		if !c.OK && c.Remediation != "" {
			_, _ = fmt.Fprintf(a.Out, "       %s\n", c.Remediation)
		}
	}
	if !allOK {
		return fmt.Errorf("doctor found problems")
	}
	return nil
}

func (a *App) checkJJInstalled() doctorCheck {
	const name = "jj installed"
	if err := jj.Installed(a.Runner); err != nil {
		return doctorCheck{
			Name:        name,
			Detail:      err.Error(),
			Remediation: "install jujutsu: https://github.com/jj-vcs/jj",
		}
	}
	return doctorCheck{Name: name, OK: true}
}

func checkColocated(mainRoot string) doctorCheck {
	const name = "repo colocated with git"
	if !jj.IsColocated(mainRoot) {
		return doctorCheck{
			Name:        name,
			Detail:      mainRoot + " does not have both .jj and .git",
			Remediation: "colocate the repo: jj git init --colocate (or clone with jj git clone --colocate)",
		}
	}
	return doctorCheck{Name: name, OK: true}
}

func (a *App) checkTmux() doctorCheck {
	const name = "tmux running"
	if a.Getenv("TMUX") == "" {
		return doctorCheck{
			Name:        name,
			Detail:      "not inside a tmux session",
			Remediation: "jumux must be run inside tmux; start a session with `tmux`",
		}
	}
	if err := tmuxctl.ServerRunning(a.Runner); err != nil {
		return doctorCheck{
			Name:        name,
			Detail:      err.Error(),
			Remediation: "ensure the tmux server is reachable (tmux list-sessions)",
		}
	}
	return doctorCheck{Name: name, OK: true}
}

func (a *App) checkBaseRevision(ctx *repoContext) doctorCheck {
	rev := ctx.Config.BaseRevision
	name := fmt.Sprintf("base_revision %q resolves", rev)
	if err := jj.ResolveRevision(a.Runner, ctx.MainRoot, rev); err != nil {
		return doctorCheck{
			Name:        name,
			Detail:      err.Error(),
			Remediation: "set base_revision in .jumux.toml or the global config to a revset that resolves in this repo",
		}
	}
	return doctorCheck{Name: name, OK: true}
}

func (a *App) checkGH() doctorCheck {
	name := forge.GitHub.Bin + " installed"
	if _, err := a.Runner.Run("", forge.GitHub.Bin, "--version"); err != nil {
		return doctorCheck{
			Name:        name,
			Detail:      err.Error(),
			Remediation: "install the GitHub CLI to use jumux pr: https://cli.github.com",
			Advisory:    true,
		}
	}
	return doctorCheck{Name: name, OK: true}
}

func (a *App) checkGlab() doctorCheck {
	name := forge.GitLab.Bin + " installed"
	if _, err := a.Runner.Run("", forge.GitLab.Bin, "--version"); err != nil {
		return doctorCheck{
			Name:        name,
			Detail:      err.Error(),
			Remediation: "install the GitLab CLI to use jumux mr: https://gitlab.com/gitlab-org/cli",
			Advisory:    true,
		}
	}
	return doctorCheck{Name: name, OK: true}
}

func (a *App) checkClaudeHooks() doctorCheck {
	const name = "Claude Code hooks installed"
	if a.ClaudeSettings == "" {
		return doctorCheck{Name: name, Detail: "no Claude settings path configured"}
	}
	missing, err := missingClaudeHookEvents(a.ClaudeSettings)
	if err != nil {
		return doctorCheck{
			Name:        name,
			Detail:      err.Error(),
			Remediation: "fix " + a.ClaudeSettings,
		}
	}
	if len(missing) > 0 {
		events := make([]string, len(missing))
		for i, he := range missing {
			events[i] = he.Event
			if he.Matcher != "" {
				events[i] += " (" + he.Matcher + ")"
			}
		}
		return doctorCheck{
			Name:        name,
			Detail:      "missing: " + strings.Join(events, ", "),
			Remediation: "run `jumux add` in a repo to be offered the hook install, or add them to " + a.ClaudeSettings + " manually",
		}
	}
	return doctorCheck{Name: name, OK: true}
}
