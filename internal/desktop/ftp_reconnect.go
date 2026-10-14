package desktop

// Reconnect the file channel using this session's in-memory login credentials.
// Replacing fileState makes late reads/transfers unable to overwrite the new view.
func (a *App) reconnectFTPFiles(p *pane) {
	if p == nil || p.closed || p.ftp == nil {
		return
	}
	directory := p.profile.InitialDirectory
	if directory == "" {
		directory = "/"
	}
	filter := ""
	if old := p.files; old != nil {
		if old.dir != "" {
			directory = old.dir
		}
		filter = old.filter
		if old.readCancel != nil {
			old.readCancel()
		}
		if old.cancel != nil {
			old.cancel()
		}
		old.activeTransfer = nil
		if old.client != nil {
			p.disposal.Add(1)
			go func() {
				defer p.disposal.Done()
				old.client.Close()
				if old.lease != nil {
					old.lease()
				}
			}()
		}
	}
	p.files = &fileState{dir: directory, input: directory, filter: filter}
	p.status = "正在重新连接 FTP…"
	a.loadFiles(p, directory)
}
