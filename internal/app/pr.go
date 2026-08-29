package app

import "github.com/richardcase/jumux/internal/forge"

// PR pushes feature's bookmark and opens a GitHub pull request for it.
// If feature is empty, it is inferred from the current workspace/window.
func (a *App) PR(feature string) error {
	return a.createOnHost(forge.GitHub, feature)
}
