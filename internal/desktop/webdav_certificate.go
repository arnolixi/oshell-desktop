package desktop

import (
	"context"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"time"
)

func (a *App) webDAVCertificateSettings(c *ui.Context) {
	f := &a.features.sharedSync
	if core.IsHTTPWebDAV(f.options.Address) {
		ui.Text(c, "HTTP 连接未加密：WebDAV 账号密码会明文传输；会话数据文件仍由主密码加密。建议仅在可信网络使用。").Wrap().TextColor(c.Theme().Text)
		return
	}
	ui.Row(c).Gap(8).Children(func() {
		ui.Button(c, "查看 / 信任服务器证书…").Disabled(f.probing || f.busy).OnClick(a.inspectWebDAVCertificate)
		if f.options.TrustedServerCertificate != (core.WebDAVCertificate{}) {
			ui.Button(c, "取消证书信任").Disabled(f.probing || f.busy).OnClick(func() {
				f.options.TrustedServerCertificate = core.WebDAVCertificate{}
				f.probeStatus = "尚未应用：保存后恢复系统证书校验"
			})
		}
	})
	text := "默认使用系统证书校验；额外信任仅保存于本机，不参与同步。"
	if f.options.TrustedServerCertificate != (core.WebDAVCertificate{}) {
		text = "已配置精确证书信任；更换服务器或证书后需重新确认。修改须点击应用保存。"
	}
	ui.Text(c, text).Wrap().TextColor(c.Theme().TextMuted)
}

func (a *App) inspectWebDAVCertificate() {
	f := &a.features.sharedSync
	if f.probing || f.busy {
		return
	}
	options := f.options
	f.probeGeneration++
	generation := f.probeGeneration
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	f.probeCancel, f.probing = cancel, true
	f.probeStatus = "正在读取证书（不发送账号密码）…"
	go func() {
		cert, err := core.InspectWebDAVCertificate(ctx, options.Address)
		cancel()
		a.post(func() {
			if generation != f.probeGeneration || !a.settingsOpen {
				return
			}
			f.probing, f.probeCancel = false, nil
			if options != f.options {
				f.probeStatus = "设置已改变，请重新查看证书"
				return
			}
			if err != nil {
				f.probeStatus = err.Error()
				return
			}
			detail := cert.Description() + "\n\n请通过可信渠道核对指纹。信任只绑定当前主机、端口和完整证书。IP 直连可能允许名称或 HTTPS 用途例外，域名连接仍严格验证。"
			valid := cert.Validate(options.Address)
			buttons := []string{"取消", "信任此证书"}
			if valid != nil {
				buttons = []string{"关闭"}
				detail += "\n\n" + valid.Error()
			}
			parent := a.preferencesWindow
			go func() {
				result, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: parent, Type: mygo.MessageWarning, Title: "核对 WebDAV 服务器证书", Message: "仅在确认服务器身份后信任", Detail: detail, Buttons: buttons, DefaultButton: 0, CancelButton: 0})
				a.post(func() {
					if err != nil || result.Button != 1 || valid != nil || !a.settingsOpen || generation != f.probeGeneration || f.options != options {
						return
					}
					if err := cert.Validate(options.Address); err != nil {
						f.probeStatus = err.Error()
						return
					}
					f.options.TrustedServerCertificate = cert
					f.probeStatus = "证书信任已暂存，点击应用后保存"
				})
			}()
		})
	}()
}
