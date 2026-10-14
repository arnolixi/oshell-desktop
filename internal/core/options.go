// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode"
)

type ProxySettings struct {
	SSHAuthentication string `json:"sshAuthentication,omitempty"`
	ID                string `json:"id"`
	Kind              string `json:"kind"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username"`
	IdentityFile      string `json:"identityFile,omitempty"`
}
type KeepAliveSettings struct {
	Enabled      bool   `json:"enabled"`
	Interval     int    `json:"interval"`
	MaxMissed    int    `json:"maxMissed"`
	TCP          bool   `json:"tcp"`
	IdleEnabled  bool   `json:"idleEnabled"`
	IdleInterval int    `json:"idleInterval"`
	IdleText     string `json:"idleText"`
}

func DefaultKeepAlive() KeepAliveSettings {
	return KeepAliveSettings{Enabled: true, Interval: 30, MaxMissed: 3, TCP: true, IdleInterval: 3600, IdleText: `\n`}
}
func (k *KeepAliveSettings) UnmarshalJSON(b []byte) error {
	type plain KeepAliveSettings
	v := plain(DefaultKeepAlive())
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*k = KeepAliveSettings(v)
	return nil
}
func (k KeepAliveSettings) Validate() error {
	if k.Interval < 1 || k.Interval > 86400 || k.MaxMissed < 1 || k.MaxMissed > 100 || k.IdleInterval < 1 || k.IdleInterval > 86400 {
		return fmt.Errorf("保活间隔需为 1–86400 秒，未响应次数为 1–100")
	}
	if k.IdleEnabled {
		_, e := IdleBytes(k.IdleText)
		return e
	}
	return nil
}
func IdleBytes(s string) ([]byte, error) {
	var b strings.Builder
	escaped := false
	for _, r := range s {
		if escaped {
			switch r {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'e':
				b.WriteByte(27)
			case '\\':
				b.WriteByte('\\')
			default:
				return nil, fmt.Errorf("不支持的空闲字符串转义 \\%c", r)
			}
			escaped = false
		} else if r == '\\' {
			escaped = true
		} else {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if escaped || len(out) == 0 || len(out) > 1024 || strings.ContainsRune(out, 0) {
		return nil, fmt.Errorf("空闲字符串为空、过长或转义不完整")
	}
	return []byte(out), nil
}

type TunnelRule struct {
	ID              string `json:"id"`
	Enabled         bool   `json:"enabled"`
	Kind            string `json:"kind"`
	BindHost        string `json:"bindHost"`
	ListenPort      int    `json:"listenPort"`
	DestinationHost string `json:"destinationHost"`
	DestinationPort int    `json:"destinationPort"`
	Note            string `json:"note"`
}

func NewTunnel() TunnelRule {
	return TunnelRule{ID: NewID(), Enabled: true, Kind: "local", BindHost: "127.0.0.1", ListenPort: 8080, DestinationHost: "localhost", DestinationPort: 80}
}
func validHost(s string) bool {
	return s != "" && !strings.HasPrefix(s, "-") && !strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(`/\\@`, r)
	})
}
func validPort(p int) bool { return p >= 1 && p <= 65535 }
func (p ProxySettings) Validate() error {
	switch p.SSHAuthentication {
	case "", "automatic", "password", "privateKey":
	default:
		return fmt.Errorf("代理 SSH 认证方式无效")
	}
	if p.Kind == "jump" && p.SSHAuthentication == "privateKey" && p.IdentityFile == "" {
		return fmt.Errorf("请选择跳板机私钥")
	}
	if p.Kind == "" || p.Kind == "none" {
		return nil
	}
	switch p.Kind {
	case "jump", "socks4", "socks4a", "socks5", "http":
	default:
		return fmt.Errorf("未知代理类型 %q", p.Kind)
	}
	if !validHost(p.Host) || !validPort(p.Port) {
		return fmt.Errorf("代理主机或端口无效")
	}
	if len(p.Username) > 255 || strings.ContainsFunc(p.Username, unicode.IsControl) {
		return fmt.Errorf("代理用户名无效")
	}
	if p.Kind == "http" && strings.Contains(p.Username, ":") {
		return fmt.Errorf("HTTP 代理用户名不能包含冒号")
	}
	if p.Kind == "jump" && strings.TrimSpace(p.Username) == "" {
		return fmt.Errorf("请填写跳板机用户名")
	}
	return nil
}
func (t TunnelRule) Validate() error {
	if !validHost(t.BindHost) || !validPort(t.ListenPort) {
		return fmt.Errorf("隧道监听地址或端口无效")
	}
	switch t.Kind {
	case "dynamic":
		return nil
	case "local", "remote":
		if !validHost(t.DestinationHost) || !validPort(t.DestinationPort) {
			return fmt.Errorf("隧道目标地址或端口无效")
		}
		return nil
	}
	return fmt.Errorf("未知隧道类型 %q", t.Kind)
}
func parseJump(s, defaultUser string) (Profile, error) {
	p := NewProfile()
	p.Group = ""
	p.Name = "跳板机"
	p.Username = defaultUser
	host := s
	if i := strings.LastIndex(s, "@"); i >= 0 {
		p.Username = s[:i]
		host = s[i+1:]
	}
	p.Host = host
	p.Port = 22
	if strings.Contains(host, ":") {
		h, port, e := net.SplitHostPort(host)
		if e == nil {
			n, e := strconv.Atoi(port)
			if e != nil {
				return p, e
			}
			p.Host = h
			p.Port = n
		} else if net.ParseIP(strings.Trim(host, "[]")) != nil {
			p.Host = strings.Trim(host, "[]")
		} else {
			return p, fmt.Errorf("跳板机格式应为 user@host:port，IPv6 使用 [地址]:端口")
		}
	}
	return p, p.Validate()
}
