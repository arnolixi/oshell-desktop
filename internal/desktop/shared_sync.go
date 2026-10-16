package desktop

import (
	"context"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"time"
)

type sharedSyncState struct {
	deferred                             bool
	deferredBackground                   bool
	savedOptions                         core.SyncOptions
	directoryChanged                     bool
	directoryDisabled                    bool
	directoryGeneration                  uint64
	manualQueued                         bool
	probeCancel                          context.CancelFunc
	probeGeneration                      uint64
	probeStatus                          string
	probing                              bool
	pendingDirectory                     *core.PendingSyncDirectory
	queuedIdentity                       string
	logExportBusy                        bool
	diagnostics                          core.SyncDiagnostics
	summary                              core.SyncSummary
	phase                                core.SyncEvent
	logEntries                           []core.SyncDiagnosticEntry
	logIssue                             string
	logsOpen, savedEnabled, pendingLocal bool
	logList                              ui.ListState
	conflictIndex                        int
	conflictDetailKey                    string
	conflictList                         ui.ListState
	options                              core.SyncOptions
	password, status                     string
	loaded, busy, conflictOpen           bool
	pendingDraft                         bool
	background                           bool
	cancel                               context.CancelFunc
	plan                                 *core.SharedSyncPlan
	choices                              map[string]core.SharedMergeChoice
	master                               []byte
}

func (a *App) loadSharedSyncSettings() {
	f := &a.features.sharedSync
	if f.loaded {
		return
	}
	f.loaded = true
	options, e := a.loadSyncOptions()
	f.options = options
	f.savedOptions = options
	f.pendingDirectory, _ = core.LoadPendingSyncDirectory(a.store.Dir)
	f.directoryChanged = false
	f.directoryDisabled = false
	f.savedEnabled = options.Enabled
	f.diagnostics = core.SyncDiagnostics{Directory: a.store.Dir, Target: options.Target(), Backend: options.Backend}
	a.loadSyncDiagnostics()
	f.pendingDraft = (core.SharedConflictStore{Directory: a.store.Dir, Target: options.Target()}).Exists()
	if e != nil {
		f.status = e.Error()
	}
}
func (a *App) sharedSyncSettings(c *ui.Context) {
	a.loadSharedSyncSettings()
	f := &a.features.sharedSync
	a.syncLocationSettings(c)
	a.syncStatusCard(c)
	ui.Text(c, "配置同步").Bold()
	ui.Text(c, "会话、设置和保存的密码以主密码加密后同步。首次连接已有数据时，由你选择保留的版本。").Wrap()
	ui.Checkbox(c, &f.options.Enabled, "启用配置同步")
	selectValue(c, "同步方式", &f.options.Backend, [][2]string{{"webdav", "WebDAV"}, {"directory", "iCloud / 同步目录"}})
	ui.Field(c, "目录地址", func() { ui.TextInput(c, &f.options.Address).Label("同步目录地址") })
	if f.options.Backend == "directory" {
		ui.Button(c, "选择目录…").Disabled(f.busy).OnClick(func() {
			paths, e := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.preferencesWindow, Title: "选择共享配置目录", Directory: true})
			if e != nil {
				f.status = e.Error()
			} else if len(paths) > 0 {
				f.options.Address = paths[0]
			}
		})
	} else {
		ui.Field(c, "用户名", func() { ui.TextInput(c, &f.options.Username) })
		ui.Field(c, "密码", func() { passwordInput(c, &f.password).Placeholder("留空沿用此地址已保存的密码") })
	}
	ui.Row(c).Gap(8).Children(func() {
		if f.options.Backend == "webdav" {
			ui.Button(c, "测试 WebDAV 连接").Disabled(f.probing || f.busy || a.configurationBusy()).OnClick(a.testWebDAVConnection)
			if f.probing {
				ui.Button(c, "取消连接测试").OnClick(a.cancelWebDAVProbe)
			}
		}
		ui.PrimaryButton(c, "关闭设置后同步").Disabled(f.busy || a.configurationBusy()).OnClick(a.queueSavedSharedSync)
		if f.busy || f.manualQueued || f.deferred {
			ui.Button(c, "取消本次").OnClick(func() {
				f.manualQueued = false
				f.deferred = false
				f.status = "本次同步已取消，本地修改保留"
				if f.cancel != nil {
					f.cancel()
				}
			})
		}
		if (f.plan != nil || f.pendingDraft) && !f.busy {
			ui.Button(c, "处理待确认差异").OnClick(a.startSavedSharedSync)
			ui.Button(c, "放弃本机冲突草稿…").OnClick(a.discardSharedConflictDraft)
		}
	})
	if f.probeStatus != "" {
		ui.Text(c, f.probeStatus).Wrap()
	}
	if f.status != "" {
		ui.Text(c, f.status).Wrap()
	}
	ui.Text(c, "目录同步的云端上传进度请在 Finder 中查看。").TextColor(c.Theme().TextMuted)
}
func (a *App) saveSharedSyncSettings() { a.saveSharedSyncSettingsThen(nil) }

func (a *App) saveSharedSyncSettingsThen(done func()) {
	f := &a.features.sharedSync
	if f.busy || a.configurationBusy() {
		return
	}
	options, password := f.options, f.password
	options.Version = 1
	if options.Backend != "directory" {
		options.RequireExisting = false
	}
	protected, locked := a.vault.State()
	if options.Enabled && (!protected || locked) {
		f.status = "请先启用并解锁主密码保护"
		return
	}
	if options.Enabled {
		if _, e := options.Transport(password, a.store.Dir); e != nil {
			f.status = e.Error()
			return
		}
	}
	f.busy = true
	a.setConfigurationBusy(true)
	go func() {
		var err error
		if options.Enabled && options.Backend == "webdav" && password != "" {
			p := options.CredentialProfile()
			err = a.vault.PutFor(p, password, p.Identity())
		}
		if err == nil {
			err = a.saveSyncOptions(options)
			if err == nil {
				event := core.SyncConfigured
				if !options.Enabled {
					event = core.SyncDisabled
				}
				_ = (core.SyncDiagnostics{Directory: a.store.Dir, Target: options.Target(), Backend: options.Backend}).Record(event, true, 0, nil, "")
			}
		}
		a.post(func() {
			f.busy = false
			a.setConfigurationBusy(false)
			if err != nil {
				f.status = err.Error()
				a.features.settingsError = "同步设置保存失败：" + err.Error()
				return
			}
			f.status = "同步设置已保存"
			if f.password == password {
				f.password = ""
			}
			if a.hub != nil {
				a.hub.scheduleSharedSync(2 * time.Second)
			}
			a.clearSharedSyncPlan()
			f.loaded = false
			a.loadSharedSyncSettings()
			if done != nil {
				done()
			}
		})
	}()
}
func (a *App) clearSharedSyncPlan() {
	f := &a.features.sharedSync
	clear(f.master)
	f.master = nil
	f.plan = nil
	f.choices = nil
	f.conflictIndex = 0
	f.conflictDetailKey = ""
	f.conflictList = ui.ListState{}
	f.conflictOpen = false
}
func (a *App) startSharedSync() {
	f := &a.features.sharedSync
	if f.busy || a.configurationBusy() {
		return
	}
	background := f.background
	f.background = false
	if a.deferSyncForSettings(background) {
		return
	}
	f.deferred = false
	options, e := a.loadSyncOptions()
	if e != nil {
		f.status = e.Error()
		return
	}
	if !options.Enabled {
		f.status = "请先保存并启用同步设置"
		return
	}
	master, e := a.vault.MasterSecret()
	if e != nil {
		f.status = e.Error()
		return
	}
	password := ""
	if options.Backend == "webdav" {
		p := options.CredentialProfile()
		password, e = a.vault.GetFor(p, p.Identity())
		if e != nil {
			clear(master)
			f.status = e.Error()
			return
		}
	}
	transport, e := options.Transport(password, a.store.Dir)
	if e != nil {
		clear(master)
		f.status = e.Error()
		return
	}
	a.clearSharedSyncPlan()
	f.master = master
	f.busy = true
	f.status = "正在读取共享配置…"
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	cfg := a.config.Clone()
	engine := &core.SharedSyncEngine{Manual: !background, Store: a.store, Vault: a.vault, Transport: transport, Replica: core.SharedReplicaStore{Directory: a.store.Dir, Target: options.Target()}}
	f.diagnostics = engine.Diagnostics()
	engine.Progress = func(event core.SyncEvent, issue error) {
		a.post(func() {
			f.phase = event
			if issue != nil {
				f.logIssue = issue.Error()
			} else {
				f.logIssue = ""
			}
		})
	}
	go func() {
		plan, err := engine.Prepare(ctx, cfg, master, !options.RequireExisting)
		if !a.post(func() {
			f.busy = false
			if err == nil && ctx.Err() != nil {
				err = ctx.Err()
			}
			cancel()
			f.cancel = nil
			f.pendingDraft = engine.ConflictStore().Exists()
			a.loadSyncDiagnostics()
			if err != nil {
				f.status = err.Error()
				if !background {
					a.status = f.status
				}
				a.clearSharedSyncPlan()
				return
			}
			if a.deferSyncForSettings(background) {
				return
			}
			f.plan = plan
			f.choices = map[string]core.SharedMergeChoice{}
			if plan.InitialChoiceRequired || len(plan.Conflicts) > 0 {
				f.conflictOpen = !background
				f.status = "共享配置有待确认差异"
				a.status = f.status
				return
			}
			a.commitSharedSync()
		}) {
			clear(master)
		}
	}()
}
func (a *App) commitSharedSync() {
	f := &a.features.sharedSync
	if f.busy || f.plan == nil || a.configurationBusy() {
		return
	}
	f.conflictOpen = false // The user has submitted this conflict dialog.
	if a.deferSyncForSettings(false) {
		return
	}
	f.busy = true
	f.conflictOpen = false
	f.status = "正在提交共享配置…"
	a.setConfigurationBusy(true)
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	plan, master := f.plan, append([]byte(nil), f.master...)
	choices := map[string]core.SharedMergeChoice{}
	for k, v := range f.choices {
		choices[k] = v
	}
	go func() {
		_, err := plan.Commit(ctx, choices, master)
		clear(master)
		cancel()
		// Reload disk even on a late replica-write failure: configuration may have
		// committed already, and the UI must not keep an obsolete copy.
		saved, loadErr := a.store.Load()
		complete := func() {
			f.busy = false
			f.cancel = nil
			a.setConfigurationBusy(false)
			a.clearSharedSyncPlan()
			f.pendingDraft = plan.ConflictDraftExists()
			a.loadSyncDiagnostics()
			if loadErr == nil {
				a.applyCommittedConfig(saved)
			}
			if err != nil {
				f.status = err.Error()
			} else if loadErr != nil {
				f.status = loadErr.Error()
			} else {
				f.status = "同步完成"
			}
			a.status = f.status
		}
		if !a.post(complete) {
			// A native/forced window close must not strand the shared busy
			// flag. The application UI loop can outlive this window.
			mygo.RunOnMain(complete)
		}
	}()
}
func (a *App) sharedSyncConflictDialog(c *ui.Context) {
	f := &a.features.sharedSync
	ui.Modal(c, &f.conflictOpen, func() {
		if f.plan == nil {
			return
		}
		width, height := c.Size()
		ui.Column(c).Width(min(float32(1000), max(float32(520), width-48))).Gap(12).Children(func() {
			ui.Text(c, "共享配置差异").FontSize(20).Bold()
			if f.plan.InitialChoiceRequired {
				ui.Text(c, fmt.Sprintf("共享端有 %d 个会话，本机有 %d 个。首次同步不会按时间自动覆盖。", len(f.plan.Remote.Profiles), len(f.plan.Local.Profiles))).Wrap()
				for _, option := range []struct {
					title string
					merge bool
				}{{"使用共享版本", false}, {"合并本机会话", true}} {
					ui.Button(c, option.title).OnClick(func() {
						if e := f.plan.ChooseInitial(option.merge); e != nil {
							f.status = e.Error()
							return
						}
						if len(f.plan.Conflicts) == 0 {
							a.commitSharedSync()
						}
					})
				}
			} else {
				a.sharedConflictComparison(c, min(float32(390), max(float32(180), height-200)))
				ui.PrimaryButton(c, "应用选择并同步").Disabled(a.sharedConflictChoiceCount() != len(f.plan.Conflicts)).OnClick(a.commitSharedSync)
			}
			ui.Button(c, "稍后处理").OnClick(func() {
				f.conflictOpen = false
				f.status = "有待确认差异，尚未同步"
				if err := f.diagnostics.Record(core.SyncPostponed, true, 0, nil, ""); err != nil {
					f.logIssue = err.Error()
				}
			})
		})
	})
}

// One timer per application, event-debounced after edits and otherwise once a
// minute, matching the source app. Disabled sync leaves no timer running.
func (h *applicationHub) scheduleSharedSync(delay time.Duration) {
	if h.syncTimer != nil {
		h.syncTimer.Stop()
		h.syncTimer = nil
	}
	windows := h.configurationWindows()
	if h.quitting || len(windows) == 0 {
		return
	}
	options, e := core.SyncOptionsStatus(h.store.Dir)
	if e != nil || !options.Enabled {
		return
	}
	owner := windows[0]
	h.syncTimer = time.AfterFunc(delay, func() {
		owner.post(func() {
			h.syncTimer = nil
			_, locked := owner.vault.State()
			if !locked && !owner.syncEditingBlocked() && !owner.configurationBusy() && !owner.features.sharedSync.busy && owner.features.sharedSync.plan == nil {
				owner.features.sharedSync.background = true
				owner.startSharedSync()
			}
			h.scheduleSharedSync(time.Minute)
		})
	})
}

func (a *App) discardSharedConflictDraft() {
	f := &a.features.sharedSync
	if f.busy || a.configurationBusy() {
		return
	}
	options, err := a.loadSyncOptions()
	if err != nil {
		f.status = err.Error()
		return
	}
	parent := a.preferencesWindow
	if parent == nil {
		parent = a.win
	}
	result, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: parent, Type: mygo.MessageWarning, Title: "放弃本机同步草稿？", Message: "仅删除本机尚未确认的冲突草稿，本机已保存配置和共享数据保持不变。", Buttons: []string{"取消", "放弃草稿"}, DefaultButton: 0, CancelButton: 0})
	if err != nil {
		f.status = err.Error()
		return
	}
	if result.Button != 1 {
		return
	}
	if err = (core.SharedConflictStore{Directory: a.store.Dir, Target: options.Target()}).Clear(); err != nil {
		f.status = err.Error()
		return
	}
	a.clearSharedSyncPlan()
	f.pendingDraft = false
	f.status = "已放弃本机冲突草稿，共享数据未修改"
}

func (a *App) startSavedSharedSync() {
	if a.features.sharedSync.busy || a.configurationBusy() {
		return
	}
	a.features.sharedSync.manualQueued = false
	a.features.sharedSync.deferred = false
	a.closeSettings()
	a.features.sharedSync.background = false
	a.startSharedSync()
}

// Menu commands use the saved sync target, never the settings draft.
func (a *App) runSyncMenuAction(discard bool) {
	if a.passwordLocked() {
		a.features.startupUnlock = true
		a.showVault()
		return
	}
	if a.configurationBusy() || a.features.sharedSync.busy {
		return
	}
	if discard {
		a.discardSharedConflictDraft()
	} else {
		a.startSavedSharedSync()
	}
	// Surface immediate setup/read errors even when preferences are closed.
	a.status = a.features.sharedSync.status
}

func (a *App) queueSavedSharedSync() {
	f := &a.features.sharedSync
	if f.busy || a.configurationBusy() {
		return
	}
	f.manualQueued = true
	f.status = "已安排同步，关闭设置后使用已保存的配置执行"
}
