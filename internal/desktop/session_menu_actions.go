package desktop

import "oshell-desktop/internal/core"

func (a *App) sessionMenuProfile() (core.Profile, bool) {
	rows := a.managerRows()
	if a.managerRow >= 0 && a.managerRow < len(rows) {
		row := rows[a.managerRow]
		if row.folder || row.parent {
			return core.Profile{}, false
		}
		return a.profileByID(row.profile.ID)
	}
	return core.Profile{}, false
}
func (a *App) editSelectedSessionFromMenu() {
	if a.passwordLocked() {
		a.showVault()
		return
	}
	if p, ok := a.sessionMenuProfile(); ok && p.Kind != "local" {
		a.editManagerProfile(p)
	}
}
func (a *App) deleteSelectedSessionFromMenu() {
	if a.passwordLocked() {
		a.showVault()
		return
	}
	if p, ok := a.sessionMenuProfile(); ok {
		// Native Session > Delete addresses the source profile, even for a link row.
		// Link-only deletion remains the session manager's explicit row action.
		a.confirmSessionDeletion([]sessionRow{{id: p.ID, name: p.Name, group: p.Group, profile: p}})
	}
}

func (a *App) connectSelectedSessionFromMenu() {
	if p, ok := a.sessionMenuProfile(); ok {
		a.request(p, 0)
		return
	}
	a.openManager()
}
