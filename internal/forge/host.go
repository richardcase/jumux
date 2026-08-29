package forge

import "strings"

// Host describes the CLI of one code-hosting service (GitHub via gh,
// GitLab via glab). All forge-specific knowledge — binary name,
// subcommand, flag spellings and the "already exists" marker — lives here
// so jumux pr and jumux mr stay identical apart from the Host they pass.
type Host struct {
	// Bin is the host CLI binary name.
	Bin string
	// CreateArgs is the subcommand that opens a change request.
	CreateArgs []string
	// HeadFlag names the source branch/bookmark to open the request from.
	HeadFlag string
	// BodyFlag names the description flag.
	BodyFlag string
	// ExistsMarker is a host-specific substring identifying the
	// "a request already exists for this branch" failure.
	ExistsMarker string
	// Noun is the human-readable name of the change request.
	Noun string
}

// GitHub is the gh CLI.
var GitHub = Host{
	Bin:          "gh",
	CreateArgs:   []string{"pr", "create"},
	HeadFlag:     "--head",
	BodyFlag:     "--body",
	ExistsMarker: "a pull request for branch",
	Noun:         "pull request",
}

// GitLab is the glab CLI.
var GitLab = Host{
	Bin:          "glab",
	CreateArgs:   []string{"mr", "create"},
	HeadFlag:     "--source-branch",
	BodyFlag:     "--description",
	ExistsMarker: "already exists",
	Noun:         "merge request",
}

// CreateArgv is the full argument list for opening a change request for
// feature with the given title and body.
func (h Host) CreateArgv(feature, title, body string) []string {
	argv := make([]string, 0, len(h.CreateArgs)+6)
	argv = append(argv, h.CreateArgs...)
	return append(argv, h.HeadFlag, feature, "--title", title, h.BodyFlag, body)
}

// AlreadyExists reports whether err from CreateArgv means a change request
// for the branch already exists, which jumux treats as success.
//
// run.ExecRunner folds the whole argv into the error string, so the title
// and body we passed are part of err.Error(). They are removed before the
// host marker is looked for: otherwise any failure (expired auth, no
// network, missing binary) for a change whose title or body happens to
// contain the marker text would be silently reported as success.
func (h Host) AlreadyExists(err error, title, body string) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	for _, injected := range []string{title, body} {
		if injected != "" {
			text = strings.ReplaceAll(text, injected, "")
		}
	}
	return strings.Contains(text, h.ExistsMarker)
}
