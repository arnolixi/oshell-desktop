package desktop

import (
	"github.com/egoist/mygo/ui"
	"strings"
)

func tabConnectionStatus(t *tab) (bool, string) {
	if t == nil || t.active == nil || t.active.closed {
		return false, "未连接"
	}
	p := t.active
	if p.term != nil && p.term.ToolConnectionActive() {
		return true, "SSH / Telnet 工具连接运行中"
	}
	if p.profile.Kind == "blank" {
		return false, "空白标签"
	}
	if p.term != nil && p.term.InLocalToolsMode() {
		return false, "已断开"
	}
	if strings.Contains(p.status, "失败") || strings.HasPrefix(p.status, "已断开") {
		return false, p.status
	}
	if p.conn != nil {
		if p.conn.Alive() {
			return true, "已连接"
		}
		return false, "已断开"
	}
	if p.profile.Kind == "local" && p.term != nil {
		if p.term.LocalProcessRunning() {
			return true, "本地终端运行中"
		}
		if p.term.HasActiveProcess() {
			return true, "正在建立本地终端"
		}
		return false, "已断开"
	}
	if p.ftp != nil && p.status == "已连接" {
		return true, "已连接"
	}
	if p.status == "正在连接…" {
		return true, "正在建立连接 / 等待认证"
	}
	return false, "未连接"
}
func preferredTabWidth(title string) float32 {
	width := float32(0)
	for _, g := range ui.Shape(title, ui.Font{Size: 12, Weight: 600}) {
		width = max(width, g.X+g.Advance)
	}
	return min(280, max(120, width+52))
}
