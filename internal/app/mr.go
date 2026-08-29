package app

import "github.com/richardcase/jumux/internal/forge"

// MR pushes feature's bookmark and opens a GitLab merge request for it.
// If feature is empty, it is inferred from the current workspace/window.
func (a *App) MR(feature string) error {
	return a.createOnHost(forge.GitLab, feature)
}
