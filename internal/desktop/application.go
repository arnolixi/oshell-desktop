// SPDX-License-Identifier: GPL-3.0-only
package desktop

import (
	"fmt"
	"io"
	"reflect"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

type applicationHub struct {
	nativeKeys            nativeShortcutMonitor
	updates               updateController
	fileWindow            *App
	externalServer        io.Closer
	externalPending       []*pendingExternalLaunch
	syncTimer             *time.Timer
	clock                 workspaceClock
	configBusy            bool
	managerClipboard      *sessionConfigurationClipboard
	settingsOwner         *App
	store                 *core.Store
	config                core.Config
	vault                 *core.Vault
	trust                 *core.TrustStore
	windows               []*App
	quitting, quitPending bool
}

func runDesktop() error {
	if e := registerFonts(); e != nil {
		return e
	}
	store, e := core.OpenStore()
	if e != nil {
		return e
	}
	cfg, e := startupConfiguration(store)
	if e != nil {
		return e
	}
	activationError := core.ActivatePendingSyncDirectory(store.Dir)
	initial := newApp(store, cfg)
	if store.StorageWarning != "" {
		initial.status = store.StorageWarning
	}
	if activationError != nil {
		initial.status = "同步目录未启用：" + activationError.Error()
	}
	if initial.vault == nil {
		return fmt.Errorf("无法打开密码库：%s", initial.status)
	}
	hub := &applicationHub{store: store, config: cfg, vault: initial.vault, trust: initial.trust}
	mygo.Theme.SetSource(mygo.ThemeSource(cfg.Settings.InterfaceTheme))
	mygo.App.WhenReady(func() {
		hub.installMenu()
		hub.attach(initial)
		hub.installDockMenu()
		hub.startExternalLaunch()
		hub.initializeUpdates()
		hub.initializeNativeShortcutCapture()
		initial.maybeResumeLocalConflict()
	})
	if runtime.GOOS == "darwin" {
		mygo.App.OnWindowAllClosed(func() {})
	}
	mygo.App.OnDidBecomeActive(hub.refreshOnActivation)
	mygo.App.OnActivate(func(visible bool) {
		if !visible && !hub.quitting {
			hub.newWindow()
			hub.refreshOnActivation()
		}
	})
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if hub.updateInstallationInProgress() {
			e.PreventDefault()
			return
		}
		if hub.quitting {
			return
		}
		count := hub.connectionCount()
		if count == 0 {
			hub.quitting = true
			return
		}
		e.PreventDefault()
		hub.confirmQuit(count)
	})
	mygo.App.OnQuit(func() {
		for _, a := range hub.windows {
			a.cancelSessionPassword()
			a.cancelLocalConflict()
		}
		if a := hub.fileWindow; a != nil {
			a.cancelSessionPassword()
			a.cancelLocalConflict()
		}

		hub.stopUpdates()
		hub.closeNativeShortcutCapture()
		if a := hub.fileWindow; a != nil {
			for _, tab := range a.tabs {
				a.walk(tab.root, a.dispose)
			}
		}
		if hub.externalServer != nil {
			hub.externalServer.Close()
		}
		hub.clock.update(nil)
		hub.store.LockConfiguration()
		hub.vault.Lock()
	})
	return mygo.App.Run()
}
func (h *applicationHub) save(source *App) bool {
	if e := h.store.Save(source.config); e != nil {
		source.status = "保存失败：" + e.Error()
		if localConflictError(e) {
			source.beginLocalConflict(source.config, nil)
		}
		return false
	}
	changedPreferences := !reflect.DeepEqual(h.config.Settings, source.config.Settings) || !reflect.DeepEqual(h.config.HighlightSets, source.config.HighlightSets)
	quickConnectionsChanged := !reflect.DeepEqual(h.config.Profiles, source.config.Profiles)
	quickCommandsChanged := !reflect.DeepEqual(h.config.QuickCommands, source.config.QuickCommands)
	shortcutsChanged := !reflect.DeepEqual(h.config.Settings.KeyboardShortcuts, source.config.Settings.KeyboardShortcuts)
	h.config = source.config.Clone()
	if (shortcutsChanged || quickConnectionsChanged || quickCommandsChanged) && mygo.App.IsReady() {
		h.installMenu()
	}
	if changedPreferences && mygo.App.IsReady() {
		mygo.Theme.SetSource(mygo.ThemeSource(h.config.Settings.InterfaceTheme))
	}
	for _, a := range h.configurationWindows() {
		if a != source {
			a.config = h.config.Clone()
			if changedPreferences {
				a.applyAppearance()
			}
		}
		if a.win != nil {
			a.win.Invalidate()
		}
		if a.manager != nil {
			a.manager.Invalidate()
		}
	}
	source.markSharedSyncChanged()
	h.scheduleSharedSync(2 * time.Second)
	h.scheduleUpdates()
	h.refreshClock()
	return true
}
func (h *applicationHub) newWindow() *App {
	a := newApp(h.store, h.config.Clone())
	a.vault = h.vault
	a.trust = h.trust
	h.attach(a)
	return a
}
func (h *applicationHub) attach(a *App) {
	a.hub = h
	if a.passwordLocked() {
		a.features.startupUnlock = true
		a.features.vaultOpen = true
		a.features.vaultAction = "unlock"
	}
	h.windows = append(h.windows, a)
	a.win = mygo.NewWindow(mygo.WindowOptions{Title: "OShell Desktop", Width: 1280, Height: 820, MinWidth: 880, MinHeight: 560, Content: ui.View(a.view)})
	a.win.OnEnterFullScreen(func() { a.fullscreenSettled(true) })
	a.win.OnLeaveFullScreen(func() { a.fullscreenSettled(false) })
	a.win.OnShow(a.refreshPaneVisibility)
	a.win.OnHide(a.refreshPaneVisibility)
	a.win.OnMinimize(a.refreshPaneVisibility)
	a.win.OnRestore(a.refreshPaneVisibility)
	a.win.OnClose(a.onWorkspaceClose)
	h.scheduleSharedSync(2 * time.Second)
	h.scheduleUpdates()
	a.win.OnClosed(func() {
		a.cancelSessionPassword()
		a.unregisterFocusWindow()
		a.stopped.Store(true)
		if a.features.sharedSync.cancel != nil {
			a.features.sharedSync.cancel()
		}
		a.clearSharedSyncPlan()
		a.closeManager()
		a.closeSettings()
		for _, t := range a.tabs {
			a.walk(t.root, a.dispose)
		}
		a.drain()
		for i, w := range h.windows {
			if w == a {
				h.windows = append(h.windows[:i], h.windows[i+1:]...)
				h.refreshClock()
				h.scheduleSharedSync(time.Minute)
				h.scheduleUpdates()
				break
			}
		}
	})
}
func (h *applicationHub) focused(w *mygo.Window) *App {
	for _, a := range h.windows {
		if w != nil && (a.win == w || a.manager == w || a.preferencesWindow == w) {
			return a
		}
	}
	for _, a := range h.windows {
		if a.win != nil && a.win.IsFocused() {
			return a
		}
	}
	if len(h.windows) > 0 {
		return h.windows[len(h.windows)-1]
	}
	return h.newWindow()
}
func (h *applicationHub) connectionCount() int {
	n := 0
	if h.fileWindow != nil {
		n += h.fileWindow.activeConnections()
	}
	for _, a := range h.windows {
		n += a.activeConnections()
	}
	return n
}
func (h *applicationHub) confirmQuit(count int) {
	if h.quitPending {
		return
	}
	h.quitPending = true
	a := h.focused(nil)
	go func() {
		r, e := mygo.Dialog.Message(mygo.MessageOptions{Parent: a.win, Type: mygo.MessageQuestion, Message: fmt.Sprintf("结束所有窗口的 %d 个活动连接并退出？", count), Detail: "正在进行的传输也会停止。", Buttons: []string{"取消", "结束并退出"}, DefaultButton: 0, CancelButton: 0})
		a.post(func() {
			h.quitPending = false
			if e == nil && r.Button == 1 {
				h.quitting = true
				mygo.App.Quit()
			}
		})
	}()
}
func (a *App) moveTabToNewWindow() {
	if a.hub == nil || !a.canMoveTabToWindow(nil) {
		return
	}
	t := a.activeTab()
	if t == nil {
		return
	}
	destination := a.hub.newWindow()
	a.transferTab(t, destination)
}
func (a *App) transferTab(t *tab, destination *App) {
	if destination == nil || destination == a || t == nil || !a.canMoveTabToWindow(destination) {
		return
	}
	at := -1
	for i, v := range a.tabs {
		if v == t {
			at = i
			break
		}
	}
	if at < 0 {
		return
	}
	sourceActive := a.activeTab()
	a.tabs = append(a.tabs[:at], a.tabs[at+1:]...)
	a.current = -1
	if sourceActive != nil && sourceActive != t {
		for i, v := range a.tabs {
			if v == sourceActive {
				a.current = i
				break
			}
		}
	}
	a.walk(t.root, func(p *pane) { a.detachInputTarget(p); p.owner.Store(destination) })
	a.pruneSyncTargets()
	retained := a.challenges[:0]
	for _, q := range a.challenges {
		if q.pane.owner.Load() == destination {
			destination.challenges = append(destination.challenges, q)
		} else {
			retained = append(retained, q)
		}
	}
	a.challenges = retained
	t.group = destination.groupForNewTab().id
	destination.tabs = append(destination.tabs, t)
	destination.setCurrentTab(t, true)
	if a.groups.root != nil {
		a.pruneGroups()
	}
	destination.current = len(destination.tabs) - 1
	if destination.win != nil {
		destination.win.Show()
		destination.win.Focus()
		destination.win.Invalidate()
	}
	if a.win != nil {
		a.win.Invalidate()
	}
	if t.active != nil {
		t.active.focus.Focus()
	}
}

func (a *App) passwordLocked() bool {
	if a.store != nil {
		if _, locked := a.store.ConfigurationProtection(); locked {
			return true
		}
	}

	if a.vault == nil {
		return false
	}
	master, locked := a.vault.State()
	return master && locked
}
