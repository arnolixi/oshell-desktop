package desktop

func reusableIdlePane(p *pane) bool {
	return p != nil && !p.closed && p.term != nil && p.term.Available() && !p.term.HasActiveProcess() && !p.transfer.active && (p.files == nil || !p.files.busy)
}
