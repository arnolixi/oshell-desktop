//go:build !darwin

package desktop

import "github.com/egoist/mygo/ui"

type updateController struct{}

func (a *App) prepareUpdateSettings()        {}
func (a *App) validateUpdateSettings() error { return nil }
func (a *App) updateSettingsView(c *ui.Context) {
	ui.Text(c, "此平台的更新流程将在 macOS 完整验收后统一完成验证。")
}
func (h *applicationHub) initializeUpdates() {}
func (h *applicationHub) scheduleUpdates()   {}
func (h *applicationHub) stopUpdates()       {}

func (h *applicationHub) updateInstallationInProgress() bool { return false }
func (h *applicationHub) updateMenuLabel() string            { return "检查更新…" }
func (a *App) invokeUpdater()                                { a.features.settingsTab = 4; a.openSettings() }

func (a *App) updatesBlockRepositoryReload() bool { return false }
