package desktop

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"strings"
)

func (a *App) addExistingLink(folder string) {
	a.closeManager()
	a.features.linkEditor = linkEditorState{open: true, mode: "addExisting", folder: folder, index: -1}
}
func (a *App) linkPickerProfiles() []core.Profile {
	terms := strings.Fields(strings.ToLower(a.features.linkEditor.query))
	var result []core.Profile
	for _, p := range a.config.Profiles {
		hay := strings.ToLower(fmt.Sprintf("%s %s %s %d %s", p.Name, p.Host, p.Username, p.Port, p.Group))
		match := true
		for _, term := range terms {
			if !strings.Contains(hay, term) {
				match = false
				break
			}
		}
		if match {
			result = append(result, p)
		}
	}
	return result
}
func (a *App) savePickedLink() {
	f := &a.features.linkEditor
	if f.id == "" {
		f.error = "请选择已有会话"
		return
	}
	if a.changeConfig(func(cfg *core.Config) error {
		for _, p := range cfg.Profiles {
			if p.ID == f.id {
				return cfg.AddLink(p, f.folder)
			}
		}
		return fmt.Errorf("原会话已不存在，请重新选择")
	}) {
		f.open = false
	} else {
		f.error = a.status
	}
}
func (a *App) linkPickerView(c *ui.Context) {
	f := &a.features.linkEditor
	ui.Text(c, "添加已有会话链接").FontSize(18).Bold()
	ui.Text(c, "目标：/Links/"+f.folder).Wrap()
	if ui.TextInput(c, &f.query).Label("搜索已有会话").Placeholder("名称、主机、用户名、端口或目录").AutoFocus().Changed() {
		f.index = -1
		f.id = ""
		f.list = ui.ListState{}
	}
	profiles := a.linkPickerProfiles()
	f.list.Selected = &f.index
	f.list.Key = func(i int) any { return profiles[i].ID }
	f.list.Label = func(i int) string { return profiles[i].Name }
	list := ui.List(c, &f.list, len(profiles), func(i int) { p := profiles[i]; ui.Text(c, p.Name+" · "+p.Host+" · /"+p.Group).SingleLine() }).Height(260)
	if f.index >= 0 && f.index < len(profiles) {
		f.id = profiles[f.index].ID
	} else {
		f.id = ""
	}
	if list.Submitted() && f.id != "" {
		a.savePickedLink()
	}
	if f.error != "" {
		ui.Text(c, f.error).TextColor(c.Theme().Danger)
	}
	ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
		ui.Button(c, "取消").OnClick(func() { f.open = false })
		ui.PrimaryButton(c, "添加链接").Disabled(f.id == "").OnClick(a.savePickedLink)
	})
}
