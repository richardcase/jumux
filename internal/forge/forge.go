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
