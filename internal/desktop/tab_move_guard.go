package desktop

var tabMoveDialogOpen = nativeTabMoveDialogOpen

func (a *App) canMoveTabToWindow(destination *App) bool {
	if a.passwordLocked() || destination != nil && destination.passwordLocked() {
		a.status = "请先解锁窗口，再移动标签"
		return false
	}
	if tabMoveDialogOpen(a.win) {
		a.status = "请先完成或取消文件选择弹窗，再移动标签"
		return false
	}
	return true
}
