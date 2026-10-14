package desktop

import "strconv"

func editorProtocolPages(kind string) ([]string, []int) {
	if kind == "ftp" {
		return []string{"连接"}, []int{0}
	}
	if kind == "sftp" {
		return []string{"连接", "代理 / 跳板机", "保持活动"}, []int{0, 1, 3}
	}
	return []string{"连接", "代理 / 跳板机", "端口转发", "保持活动"}, []int{0, 1, 2, 3}
}
func (a *App) syncEditorProtocol() {
	f := &a.features
	previous := f.editorKind
	kind := a.draft.Kind
	if previous == "" || previous == kind {
		f.editorKind = kind
		return
	}
	if f.sessionPassword.original == nil {
		before := a.config.SessionDefaults.MakeProfile(previous, "")
		after := a.config.SessionDefaults.MakeProfile(kind, "")
		if a.port == strconv.Itoa(before.Port) {
			a.port = strconv.Itoa(after.Port)
		}
		if a.draft.Username == before.Username {
			a.draft.Username = after.Username
		}
		if a.draft.InitialDirectory == before.InitialDirectory {
			a.draft.InitialDirectory = after.InitialDirectory
		}
	} else {
		before, after := 22, 22
		if previous == "ftp" {
			before = 21
		}
		if kind == "ftp" {
			after = 21
		}
		if a.port == strconv.Itoa(before) {
			a.port = strconv.Itoa(after)
		}
		if kind == "ftp" && a.draft.InitialDirectory == "." {
			a.draft.InitialDirectory = "/"
		}
	}
	f.editorKind = kind
	f.editTab = 0
}
