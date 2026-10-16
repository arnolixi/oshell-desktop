package desktop

import "github.com/egoist/mygo"

// All workspace windows must keep processing completion events while a shared
// sync is running: committing holds the application's configuration busy flag.
func (a *App) onWorkspaceClose(event *mygo.CloseEvent) {
	if a.features.localConflict.active {
		event.PreventDefault()
		a.cancelLocalConflict()
		return
	}
	if a.features.sessionPassword.busy {
		event.PreventDefault()
		a.cancelSessionPassword()
		return
	}
	if a.features.sharedSync.busy {
		event.PreventDefault()
		a.status = "配置同步正在进行，请等待完成或在同步设置中取消"
		return
	}
	if a.hub != nil && !a.hub.quitting && !a.features.allowClose && a.activeConnections() > 0 {
		event.PreventDefault()
		a.confirmExit(false)
	}
}
