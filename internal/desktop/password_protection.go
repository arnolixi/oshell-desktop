package desktop

import "github.com/egoist/mygo/ui"

func passwordProtectionField(c *ui.Context, mode *string) {
	selectValue(c, "密码保护", mode, [][2]string{{"local", "本机自动加密（无需主密码）"}, {"master", "使用主密码加密"}})
}
