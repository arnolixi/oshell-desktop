// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var uuidText = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ProxyCredentialProfile(p ProxySettings) Profile {
	out := NewProfile()
	out.ID = p.ID
	out.Name = "代理 " + p.Host
	out.Host = p.Host
	out.Port = p.Port
	out.Username = p.Username
	out.IdentityFile = p.IdentityFile
	return out
}

type PortableProfile struct {
	Definition              Profile
	Password, ProxyPassword *LegacyPassword
}

func (p PortableProfile) MarshalJSON() ([]byte, error) {
	definition := p.Definition
	definition.Tunnels = append([]TunnelRule{}, definition.Tunnels...)
	if definition.Proxy.SSHAuthentication == "" {
		definition.Proxy.SSHAuthentication = "automatic"
	}
	if definition.Proxy.ID == "" {
		// Serialization must be deterministic for sync comparison. A missing
		// disabled proxy ID is derived from the stable session ID, never randomized.
		sum := sha256.Sum256([]byte("OShell absent proxy v1\n" + strings.ToUpper(definition.ID)))
		sum[6] = (sum[6] & 15) | 0x50
		sum[8] = (sum[8] & 63) | 0x80
		definition.Proxy.ID = fmt.Sprintf("%x-%x-%x-%x-%x", sum[:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
	}
	data, e := json.Marshal(definition)
	if e != nil {
		return nil, e
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(data, &fields)
	if p.Password != nil {
		fields["encryptedPassword"], e = json.Marshal(p.Password)
		if e != nil {
			return nil, e
		}
	}
	if p.ProxyPassword != nil {
		var proxy map[string]json.RawMessage
		if e = json.Unmarshal(fields["proxy"], &proxy); e != nil {
			return nil, e
		}
		proxy["encryptedPassword"], e = json.Marshal(p.ProxyPassword)
		if e != nil {
			return nil, e
		}
		fields["proxy"], e = json.Marshal(proxy)
		if e != nil {
			return nil, e
		}
	}
	return json.Marshal(fields)
}
func (p *PortableProfile) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(data, &fields); e != nil {
		return e
	}
	for _, name := range []string{"id", "name", "group", "kind", "host", "port", "username", "identityFile", "jumpHost"} {
		if raw, ok := fields[name]; !ok || string(raw) == "null" {
			return fmt.Errorf("会话缺少必需字段 %s", name)
		}
	}
	if e := json.Unmarshal(data, &p.Definition); e != nil {
		return e
	}
	if !uuidText.MatchString(p.Definition.ID) {
		return fmt.Errorf("会话 ID 不是 UUID")
	}
	p.Definition.ID = strings.ToUpper(p.Definition.ID)
	var extras struct {
		EncryptedPassword *LegacyPassword
		Proxy             struct{ EncryptedPassword *LegacyPassword }
	}
	if e := json.Unmarshal(data, &extras); e != nil {
		return e
	}
	p.Password = extras.EncryptedPassword
	p.ProxyPassword = extras.Proxy.EncryptedPassword
	if raw := fields["proxy"]; raw != nil && string(raw) != "null" {
		if !uuidText.MatchString(p.Definition.Proxy.ID) {
			return fmt.Errorf("代理 ID 不是 UUID")
		}
		p.Definition.Proxy.ID = strings.ToUpper(p.Definition.Proxy.ID)
	}
	for _, envelope := range []*LegacyPassword{p.Password, p.ProxyPassword} {
		if envelope != nil {
			if e := validatePasswordEnvelope(*envelope); e != nil {
				return e
			}
		}
	}
	return nil
}
func validatePasswordEnvelope(e LegacyPassword) error {
	salt, se := base64.StdEncoding.DecodeString(e.Salt)
	data, de := base64.StdEncoding.DecodeString(e.Ciphertext)
	if se != nil || de != nil || e.Version != 1 || e.Algorithm != "AES-256-GCM" || e.KDF != "PBKDF2-HMAC-SHA256" || e.Iterations < 600000 || e.Iterations > 2000000 || len(salt) != 16 || len(data) < 28 || len(data) > 8192 || e.LocalKeyID != "" && !uuidText.MatchString(e.LocalKeyID) {
		return fmt.Errorf("加密密码格式无效")
	}
	return nil
}

type PortableArchive struct {
	Proxies     []PortableProxy   `json:"proxies,omitempty"`
	Format      string            `json:"format"`
	Version     int               `json:"version"`
	Profiles    []PortableProfile `json:"profiles"`
	Directories []string          `json:"directories"`
	Links       []SessionLink     `json:"links,omitempty"`
}

func (a PortableArchive) PasswordCount() int {
	n := 0
	for _, p := range a.Profiles {
		if p.Password != nil {
			n++
		}
		if p.ProxyPassword != nil {
			n++
		}
	}
	for _, p := range a.Proxies {
		if p.Password != nil {
			n++
		}
	}
	return n
}
func canonicalDirectory(value string) bool {
	normalized, e := NormalizeDirectory(value)
	return e == nil && normalized == value
}
func (a PortableArchive) Validate() error {
	if a.Format != "OShell.sessions" || a.Version < 1 || a.Version > 4 || len(a.Profiles) > 2000 || len(a.Directories) > 4000 || len(a.Links) > 4000 || a.Version < 3 && len(a.Links) > 0 {
		return fmt.Errorf("会话导出格式、版本或大小无效")
	}
	var proxyDefinitions []ProxyProfile
	for _, p := range a.Proxies {
		proxyDefinitions = append(proxyDefinitions, p.Definition)
	}
	definitions := make([]Profile, 0, len(a.Profiles))
	for _, p := range a.Profiles {
		definitions = append(definitions, p.Definition)
	}
	if a.Version < 4 && len(a.Proxies) > 0 {
		return fmt.Errorf("共享代理需要归档版本 4")
	}
	if e := ValidateProxyCatalog(proxyDefinitions, definitions); e != nil {
		return e
	}
	ids := map[string]bool{}
	for _, p := range a.Profiles {
		definition := p.Definition
		id := strings.ToUpper(definition.ID)
		if !uuidText.MatchString(id) || ids[id] {
			return fmt.Errorf("会话 ID 无效或重复")
		}
		ids[id] = true
		if e := definition.Validate(); e != nil {
			return e
		}
		if a.Version == 1 && (definition.Kind == "ftp" || definition.Kind == "sftp") {
			return fmt.Errorf("文件会话需要归档版本 2 或更高")
		}
		if !canonicalDirectory(definition.Group) {
			return fmt.Errorf("会话目录不是规范路径")
		}
		switch definition.TitleMode {
		case "activeProbe", "passive", "shellIntegration":
		default:
			return fmt.Errorf("主机名识别模式无效")
		}
	}
	for _, d := range a.Directories {
		if !canonicalDirectory(d) {
			return fmt.Errorf("归档目录不是规范路径")
		}
	}
	linkIDs, positions := map[string]bool{}, map[string]bool{}
	for _, link := range a.Links {
		id := strings.ToUpper(link.ID)
		pid := strings.ToUpper(link.ProfileID)
		position := pid + "/" + link.Folder
		if !uuidText.MatchString(id) || linkIDs[id] || positions[position] || !ids[pid] || !canonicalDirectory(link.Folder) || strings.TrimSpace(link.Name) == "" || len(link.Name) > 1024 || strings.ContainsFunc(link.Name, unicode.IsControl) {
			return fmt.Errorf("快捷引用无效、重复或源会话不存在")
		}
		linkIDs[id] = true
		positions[position] = true
	}
	return nil
}
func DecodePortableArchive(data []byte) (PortableArchive, error) {
	var result PortableArchive
	if len(data) > MaxArchive {
		return result, fmt.Errorf("会话文件超过 16 MiB")
	}
	var required map[string]json.RawMessage
	if e := json.Unmarshal(data, &required); e != nil {
		return result, e
	}
	for _, key := range []string{"format", "version", "profiles", "directories"} {
		if raw, ok := required[key]; !ok || string(raw) == "null" {
			return result, fmt.Errorf("归档缺少必需字段 %s", key)
		}
	}
	if e := json.Unmarshal(data, &result); e != nil {
		return result, e
	}
	return result, result.Validate()
}
func (a PortableArchive) Encode() ([]byte, error) {
	if e := a.Validate(); e != nil {
		return nil, e
	}
	for _, p := range a.Profiles {
		for _, envelope := range []*LegacyPassword{p.Password, p.ProxyPassword} {
			if envelope != nil && envelope.LocalKeyID != "" {
				return nil, fmt.Errorf("本机密钥密码不能直接导出")
			}
		}
	}
	for _, p := range a.Proxies {
		if p.Password != nil && p.Password.LocalKeyID != "" {
			return nil, fmt.Errorf("本机代理密钥引用不能直接导出")
		}
	}
	if a.Profiles == nil {
		a.Profiles = []PortableProfile{}
	}
	if a.Directories == nil {
		a.Directories = []string{}
	}
	data, e := json.MarshalIndent(a, "", "  ")
	if e == nil && len(data) > MaxArchive {
		e = fmt.Errorf("会话文件超过 16 MiB")
	}
	return data, e
}

type CredentialUpdate struct {
	Protection string
	Delete     bool
	Profile    Profile
	Password   string
	Identity   SSHIdentity
}

func (a PortableArchive) Merge(ctx context.Context, config Config, directory string, includePasswords bool, master []byte) (Config, []CredentialUpdate, error) {
	if e := a.Validate(); e != nil {
		return config, nil, e
	}
	prefix, e := NormalizeDirectory(directory)
	if e != nil {
		return config, nil, e
	}
	linksDestination := prefix == "Links" || strings.HasPrefix(prefix, "Links/")
	linkPrefix := ""
	if linksDestination {
		linkPrefix = strings.TrimPrefix(strings.TrimPrefix(prefix, "Links"), "/")
		prefix = ""
	}
	result := config.Clone()
	ids := map[string]string{}
	var credentials []CredentialUpdate
	join := func(a, b string) string {
		if a == "" {
			return b
		}
		if b == "" {
			return a
		}
		return a + "/" + b
	}
	bind := func(source, destination Profile, envelope *LegacyPassword) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if !includePasswords || envelope == nil {
			return nil
		}
		secret, e := DecryptMacPassword(*envelope, master, source)
		if e != nil {
			return e
		}
		credentials = append(credentials, CredentialUpdate{Profile: destination, Password: secret, Identity: envelope.Identity})
		return nil
	}
	proxyIDs := map[string]string{}
	for _, source := range a.Proxies {
		proxyIDs[strings.ToUpper(source.Definition.ID())] = NewID()
	}
	for _, source := range a.Proxies {
		next := source.Definition
		next.Settings.ID = proxyIDs[strings.ToUpper(source.Definition.ID())]
		if next.UpstreamID != "" {
			next.UpstreamID = proxyIDs[strings.ToUpper(next.UpstreamID)]
		}
		if e = bind(ProxyCredentialProfile(source.Definition.Settings), ProxyCredentialProfile(next.Settings), source.Password); e != nil {
			return config, nil, e
		}
		result.Proxies = append(result.Proxies, next)
	}
	for _, source := range a.Profiles {
		if e := ctx.Err(); e != nil {
			return config, nil, e
		}
		next := source.Definition
		next.ID = NewID()
		next.Proxy.ID = NewID()
		if next.ProxyID != "" {
			next.ProxyID = proxyIDs[strings.ToUpper(next.ProxyID)]
		}
		next.Group = join(prefix, next.Group)
		ids[strings.ToUpper(source.Definition.ID)] = next.ID
		base := next.Name
		for suffix := 1; ; suffix++ {
			collision := false
			for _, p := range result.Profiles {
				collision = collision || p.Group == next.Group && p.Name == next.Name
			}
			if !collision {
				break
			}
			tail := ""
			if suffix > 1 {
				tail = fmt.Sprint(" ", suffix)
			}
			next.Name = base + "（导入副本" + tail + "）"
		}
		if e = bind(source.Definition, next, source.Password); e != nil {
			return config, nil, e
		}
		if e = bind(ProxyCredentialProfile(source.Definition.Proxy), ProxyCredentialProfile(next.Proxy), source.ProxyPassword); e != nil {
			return config, nil, e
		}
		result.Profiles = append(result.Profiles, next)
	}
	referenced := map[string]bool{}
	for _, link := range a.Links {
		oldID := strings.ToUpper(link.ProfileID)
		referenced[oldID] = true
		link.ID = NewID()
		link.ProfileID = ids[oldID]
		link.Folder = join(linkPrefix, link.Folder)
		base := link.Name
		for n := 1; ; n++ {
			collision := false
			for _, l := range result.Links {
				collision = collision || l.Folder == link.Folder && l.Name == link.Name
			}
			if !collision {
				break
			}
			link.Name = fmt.Sprintf("%s（导入副本 %d）", base, n)
		}
		result.Links = append(result.Links, link)
	}
	for _, d := range a.Directories {
		if d == "Links" || strings.HasPrefix(d, "Links/") {
			result.Directories = append(result.Directories, join("Links", join(linkPrefix, strings.TrimPrefix(strings.TrimPrefix(d, "Links"), "/"))))
		} else if linksDestination {
			result.Directories = append(result.Directories, join("Links", join(linkPrefix, d)))
		} else {
			result.Directories = append(result.Directories, join(prefix, d))
		}
	}
	if linksDestination {
		for _, source := range a.Profiles {
			if !referenced[strings.ToUpper(source.Definition.ID)] {
				result.Links = append(result.Links, SessionLink{ID: NewID(), ProfileID: ids[strings.ToUpper(source.Definition.ID)], Name: source.Definition.Name, Folder: join(linkPrefix, source.Definition.Group)})
			}
		}
	}
	if e = result.MigrateProxyCatalog(); e != nil {
		return config, nil, e
	}
	if e = result.Validate(); e != nil {
		return config, nil, e
	}
	return result, credentials, nil
}
func (v *Vault) PortableArchive(ctx context.Context, config Config, includePasswords bool, master []byte) (PortableArchive, error) {
	result := PortableArchive{Format: "OShell.sessions", Version: 2, Directories: append([]string{}, config.Directories...), Links: append([]SessionLink(nil), config.Links...)}
	if len(result.Links) > 0 {
		result.Version = 3
	}
	if includePasswords && len([]rune(string(master))) < 8 {
		return result, fmt.Errorf("导出密码至少需要 8 个字符")
	}
	for _, p := range config.Profiles {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		value := PortableProfile{Definition: p}
		if includePasswords {
			for i, credentialProfile := range []Profile{p, ProxyCredentialProfile(p.Proxy)} {
				if i == 1 && p.ProxyID != "" {
					continue
				}
				secret, identity, e := v.exportCredential(credentialProfile)
				if e != nil {
					return result, e
				}
				if secret == "" {
					continue
				}
				envelope, e := EncryptMacPassword(secret, master, credentialProfile, identity)
				if e != nil {
					return result, e
				}
				if i == 0 {
					value.Password = &envelope
				} else {
					value.ProxyPassword = &envelope
				}
			}
		}
		result.Profiles = append(result.Profiles, value)
	}
	proxies, e := RequiredProxies(config.Profiles, config.Proxies)
	if e != nil {
		return result, e
	}
	for _, p := range proxies {
		value := PortableProxy{Definition: p}
		if includePasswords {
			profile := ProxyCredentialProfile(p.Settings)
			secret, identity, e := v.exportCredential(profile)
			if e != nil {
				return result, e
			}
			if secret != "" {
				envelope, e := EncryptMacPassword(secret, master, profile, identity)
				if e != nil {
					return result, e
				}
				value.Password = &envelope
			}
		}
		result.Proxies = append(result.Proxies, value)
	}
	if len(result.Proxies) > 0 {
		result.Version = 4
	}
	return result, result.Validate()
}
func (v *Vault) exportCredential(p Profile) (string, SSHIdentity, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	identity := p.Identity()
	entry, ok := v.data.Entries[p.ID]
	if !ok || entry.Identity != credentialIdentity(p) {
		return "", identity, nil
	}
	if len(v.key) == 0 && !entry.Local {
		return "", identity, ErrVaultLocked
	}
	plain, e := v.decryptEntryLocked(p.ID, entry)
	if e != nil {
		return "", identity, e
	}
	defer clear(plain)
	if entry.Resolved != nil {
		identity = *entry.Resolved
	}
	return string(plain), identity, nil
}
