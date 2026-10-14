package desktop

// Native menu callbacks bypass the view's modal event routing. Apply the same
// exclusion before changing workspace state; native Edit roles remain routed by
// the framework to the focused text control.
func (a *App) workspaceMenuBlocked() bool {
	return a.modalActive() || tabMoveDialogOpen(a.win) ||
		a.preferencesWindow != nil && tabMoveDialogOpen(a.preferencesWindow) ||
		a.manager != nil && tabMoveDialogOpen(a.manager)
}
