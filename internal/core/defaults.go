// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"encoding/json"
	"fmt"
)

type SessionDefaults struct {
	SSHPort      int               `json:"sshPort"`
	SSHUsername  string            `json:"sshUsername"`
	IdentityFile string            `json:"identityFile"`
	LegacySSH    bool              `json:"legacySSH"`
	SSHDirectory string            `json:"sshDirectory"`
	FTPPort      int               `json:"ftpPort"`
	FTPUsername  string            `json:"ftpUsername"`
	FTPDirectory string            `json:"ftpDirectory"`
	QuickConnect bool              `json:"quickConnect"`
	KeepAlive    KeepAliveSettings `json:"keepAlive"`
}

func DefaultSessionDefaults() SessionDefaults {
	return SessionDefaults{SSHPort: 22, SSHDirectory: ".", FTPPort: 21, FTPUsername: "anonymous", FTPDirectory: "/", QuickConnect: true, KeepAlive: DefaultKeepAlive()}
}
func (d *SessionDefaults) UnmarshalJSON(b []byte) error {
	type plain SessionDefaults
	value := plain(DefaultSessionDefaults())
	if e := json.Unmarshal(b, &value); e != nil {
		return e
	}
	*d = SessionDefaults(value)
	return nil
}
func (d SessionDefaults) MakeProfile(kind, directory string) Profile {
	p := NewProfile()
	p.Kind = kind
	p.Group = directory
	p.Name = "新 " + kind + " 会话"
	p.QuickConnect = d.QuickConnect
	if kind == "ssh" || kind == "sftp" {
		p.Port = d.SSHPort
		p.Username = d.SSHUsername
		p.IdentityFile = d.IdentityFile
		p.LegacySSH = d.LegacySSH
		p.InitialDirectory = d.SSHDirectory
		p.KeepAlive = d.KeepAlive
		if kind == "sftp" {
			p.KeepAlive.IdleEnabled = false
		}
	} else if kind == "ftp" {
		p.Port = d.FTPPort
		p.Username = d.FTPUsername
		p.InitialDirectory = d.FTPDirectory
	}
	return p
}
func (d SessionDefaults) Validate() error {
	if e := d.KeepAlive.Validate(); e != nil {
		return e
	}
	for _, kind := range []string{"ssh", "ftp"} {
		p := d.MakeProfile(kind, "")
		p.Host = "defaults.example.invalid"
		if e := p.Validate(); e != nil {
			return e
		}
	}
	return nil
}
func DuplicateProfile(p Profile, existing []Profile) Profile {
	names := map[string]bool{}
	for _, v := range existing {
		if v.Group == p.Group {
			names[v.Name] = true
		}
	}
	base := p.Name + " - 副本"
	name := base
	for n := 2; names[name]; n++ {
		name = fmt.Sprintf("%s %d", base, n)
	}
	p.Name = name
	p.ID = NewID()
	p.Proxy.ID = NewID()
	p.Tunnels = append([]TunnelRule(nil), p.Tunnels...)
	for i := range p.Tunnels {
		p.Tunnels[i].ID = NewID()
	}
	return p
}
