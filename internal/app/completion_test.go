package app

import (
	"strings"
	"testing"
)

func TestCompletionScripts(t *testing.T) {
	tests := []struct {
		shell string
		want  []string
	}{
		{shell: "bash", want: []string{"_jumux_complete", "complete -F _jumux_complete jumux", "__complete-features"}},
		{shell: "zsh", want: []string{"#compdef jumux", "_jumux \"$@\"", "__complete-features"}},
		{shell: "fish", want: []string{"complete -c jumux", "__jumux_features", "__complete-features"}},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			f := newFixture(t)
			if err := f.app.Completion(tt.shell); err != nil {
				t.Fatal(err)
			}
			out := f.out.String()
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("%s completion missing %q:\n%s", tt.shell, want, out)
				}
			}
			// Every feature-taking command should be wired into the script.
			for _, cmd := range []string{"remove", "restart", "attach", "rename"} {
				if !strings.Contains(out, cmd) {
					t.Errorf("%s completion missing command %q:\n%s", tt.shell, cmd, out)
				}
			}
		})
	}
}

func TestCompletionRejectsUnknownShell(t *testing.T) {
	f := newFixture(t)
	if err := f.app.Completion("powershell"); err == nil || !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("got %v", err)
	}
}

func TestCompletionFeaturesListsNonDefaultWorkspaces(t *testing.T) {
	f := newFixture(t)
	f.responses["jj workspace list"] = "default: qq 11\nauth: kk 22\nbilling: mm 33"
	if err := f.app.CompletionFeatures(); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "auth") || !strings.Contains(out, "billing") {
		t.Errorf("expected auth and billing, got: %q", out)
	}
	if strings.Contains(out, "default") {
		t.Errorf("default workspace must be excluded: %q", out)
	}
}

func TestCompletionFeaturesOutsideRepoIsQuiet(t *testing.T) {
	f := newFixture(t)
	f.responses["jj root"] = ""
	f.failOn = "jj root"
	if err := f.app.CompletionFeatures(); err != nil {
		t.Fatalf("expected nil error outside a repo, got %v", err)
	}
	if f.out.String() != "" {
		t.Errorf("expected no output outside a repo, got %q", f.out.String())
	}
}
