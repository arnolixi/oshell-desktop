package desktop

func (a *App) showSyncInput() {
	f := &a.features
	f.syncSelectionDraft = map[string]bool{}
	for id, selected := range f.syncTargets {
		if selected {
			f.syncSelectionDraft[id] = true
		}
	}
	if len(f.syncSelectionDraft) == 0 {
		if p := a.activePane(); p != nil {
			f.syncSelectionDraft[p.id] = true
		}
	}
	f.syncDraftEnabled = true
	f.syncDraftError = ""
	f.syncOpen = true
}
func (a *App) applySyncInputDraft() {
	f := &a.features
	chosen := map[string]bool{}
	if f.syncDraftEnabled {
		for _, p := range a.allPanes() {
			if f.syncSelectionDraft[p.id] && !p.closed {
				if p.term == nil || !p.term.ManagedInputAvailable() {
					f.syncDraftError = "所选会话暂不可输入"
					return
				}
				chosen[p.id] = true
			}
		}
		if len(chosen) == 1 {
			f.syncDraftError = "同步输入需要至少选择两个可输入的终端"
			return
		}
	}
	f.syncTargets = chosen
	f.syncEnabled = len(chosen) >= 2
	f.syncOpen = false
	f.syncDraftError = ""
}
