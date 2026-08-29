// Package forge contains logic shared by jumux's PR and MR commands:
// pushing a feature's jj bookmark and deriving a title/body from its
// change description.
package forge

import (
	"fmt"
	"strings"

	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/run"
)

// PreparePush derives a title/body from feature's change description, then
// sets feature's bookmark to its working-copy commit and pushes it.
//
// The description is read first because jj refuses to push a bookmark
// pointing at a description-less commit ("Won't push commit ... since it
// has no description"); checking up front turns that into a clear jumux
// error and leaves the bookmark untouched.
func PreparePush(r run.Runner, wsRoot, feature string) (title, body string, err error) {
	rev := feature + "@"

	desc, err := jj.Description(r, wsRoot, rev)
	if err != nil {
		return "", "", err
	}
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return "", "", fmt.Errorf("feature %q has no change description; run 'jj describe' first", feature)
	}

	if err := jj.BookmarkSet(r, wsRoot, feature, rev); err != nil {
		return "", "", err
	}
	if err := jj.GitPush(r, wsRoot, feature); err != nil {
		return "", "", err
	}

	lines := strings.SplitN(desc, "\n", 2)
	title = lines[0]
	if len(lines) == 2 {
		body = strings.TrimSpace(lines[1])
	}
	return title, body, nil
}
