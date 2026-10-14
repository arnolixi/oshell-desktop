// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"encoding/json"
	"fmt"
)

type Archive struct {
	Format      string        `json:"format"`
	Version     int           `json:"version"`
	Profiles    []Profile     `json:"profiles"`
	Directories []string      `json:"directories"`
	Links       []SessionLink `json:"links,omitempty"`
}
type Imported struct {
	Profiles    []Profile
	Directories []string
	Links       []SessionLink
	Warnings    []string
}

func ExportArchive(c Config) ([]byte, error) {
	archive := PortableArchive{Format: "OShell.sessions", Version: 2, Directories: append([]string{}, c.Directories...), Links: c.Links}
	for _, p := range c.Profiles {
		archive.Profiles = append(archive.Profiles, PortableProfile{Definition: p})
	}
	if len(archive.Links) > 0 {
		archive.Version = 3
	}
	proxies, e := RequiredProxies(c.Profiles, c.Proxies)
	if e != nil {
		return nil, e
	}
	for _, p := range proxies {
		archive.Proxies = append(archive.Proxies, PortableProxy{Definition: p})
	}
	if len(proxies) > 0 {
		archive.Version = 4
	}
	return archive.Encode()
}
func ImportMac(data []byte) ([]Profile, []string, error) {
	v, e := ImportArchive(data)
	return v.Profiles, v.Warnings, e
}
func ImportArchive(data []byte) (Imported, error) {
	var out Imported
	if len(data) > MaxArchive {
		return out, fmt.Errorf("文件超过 16 MiB")
	}
	var raw struct {
		Format      string
		Version     int
		Profiles    []json.RawMessage
		Directories []string
		Links       []SessionLink
	}
	if e := json.Unmarshal(data, &raw); e != nil {
		return out, e
	}
	if raw.Format != "OShell.sessions" || raw.Version < 1 || raw.Version > 3 {
		return out, fmt.Errorf("请选择 OShell 导出的会话 JSON（版本 1–3）")
	}
	if len(raw.Profiles) > 2000 || len(raw.Directories) > 4000 || len(raw.Links) > 4000 {
		return out, fmt.Errorf("会话文件项目过多")
	}
	ids := map[string]string{}
	seen := map[string]bool{}
	for _, b := range raw.Profiles {
		p := NewProfile()
		if e := json.Unmarshal(b, &p); e != nil {
			return out, e
		}
		old := p.ID
		if seen[old] {
			return out, fmt.Errorf("导入文件含重复会话 ID")
		}
		seen[old] = true
		if e := p.Validate(); e != nil {
			out.Warnings = append(out.Warnings, p.Name+"："+e.Error()+"，已跳过")
			continue
		}
		var extra struct {
			EncryptedPassword json.RawMessage
			Proxy             struct{ EncryptedPassword json.RawMessage }
		}
		json.Unmarshal(b, &extra)
		if (len(extra.EncryptedPassword) > 0 && string(extra.EncryptedPassword) != "null") || (len(extra.Proxy.EncryptedPassword) > 0 && string(extra.Proxy.EncryptedPassword) != "null") {
			out.Warnings = append(out.Warnings, p.Name+"：旧版加密密码未导入，首次连接需重新输入")
		}
		group, e := NormalizeDirectory(p.Group)
		if e != nil {
			return out, e
		}
		p.Group = group
		p.ID = NewID()
		for i := range p.Tunnels {
			p.Tunnels[i].ID = NewID()
		}
		ids[old] = p.ID
		out.Profiles = append(out.Profiles, p)
	}
	for _, d := range raw.Directories {
		d, e := NormalizeDirectory(d)
		if e != nil {
			return out, e
		}
		if d != "" {
			out.Directories = append(out.Directories, d)
		}
	}
	for _, l := range raw.Links {
		id, ok := ids[l.ProfileID]
		if !ok {
			out.Warnings = append(out.Warnings, l.Name+"：源会话未导入，快捷引用已跳过")
			continue
		}
		folder, e := NormalizeDirectory(l.Folder)
		if e != nil {
			return out, e
		}
		l.ID = NewID()
		l.ProfileID = id
		l.Folder = folder
		out.Links = append(out.Links, l)
	}
	return out, nil
}
