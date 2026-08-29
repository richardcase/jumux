package forge

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCreateArgv(t *testing.T) {
	tests := []struct {
		name string
		host Host
		want string
	}{
		{
			name: "github",
			host: GitHub,
			want: "pr create --head auth --title Add auth --body Body text",
		},
		{
			name: "gitlab",
			host: GitLab,
			want: "mr create --source-branch auth --title Add auth --description Body text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(tt.host.CreateArgv("auth", "Add auth", "Body text"), " ")
			if got != tt.want {
				t.Errorf("CreateArgv() = %q, want %q", got, tt.want)
			}
		})
	}
}

// execError formats err the way run.ExecRunner does: the whole argv,
// including the title and body values, is folded into the error string.
func execError(bin string, argv []string, stderr string) error {
	return fmt.Errorf("%s %s: %w: %s", bin, strings.Join(argv, " "),
		errors.New("exit status 1"), stderr)
}

func TestAlreadyExists(t *testing.T) {
	tests := []struct {
		name   string
		host   Host
		title  string
		body   string
		stderr string
		want   bool
	}{
		{
			name:   "gh reports an existing pull request",
			host:   GitHub,
			title:  "Add auth",
			stderr: `a pull request for branch "auth" into branch "main" already exists`,
			want:   true,
		},
		{
			name:   "glab reports an existing merge request",
			host:   GitLab,
			title:  "Add auth",
			stderr: "a merge request for this branch already exists",
			want:   true,
		},
		{
			name:   "gh auth failure is not success",
			host:   GitHub,
			title:  "Add auth",
			stderr: "authentication required",
			want:   false,
		},
		{
			// The adversarial case: the marker text appears only because
			// the user's own title is echoed back in the argv.
			name:   "gh failure whose title contains the marker is not success",
			host:   GitHub,
			title:  `Explain why "a pull request for branch" already exists is confusing`,
			stderr: "authentication required",
			want:   false,
		},
		{
			name:   "glab failure whose body contains the marker is not success",
			host:   GitLab,
			title:  "Add auth",
			body:   "Note: the tag already exists upstream, so skip it.",
			stderr: "authentication required",
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argv := tt.host.CreateArgv("auth", tt.title, tt.body)
			err := execError(tt.host.Bin, argv, tt.stderr)
			if got := tt.host.AlreadyExists(err, tt.title, tt.body); got != tt.want {
				t.Errorf("AlreadyExists() = %v, want %v (err = %v)", got, tt.want, err)
			}
		})
	}
}

func TestAlreadyExistsNilError(t *testing.T) {
	if GitHub.AlreadyExists(nil, "", "") {
		t.Error("AlreadyExists(nil) = true, want false")
	}
}
