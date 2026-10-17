package core

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Update credentials have a separate LOCAL vault. They are never session
// entries and are not included in configuration export or shared-vault sync.
func updateCredential(repository string) (Profile, error) {
	source, e := ParseUpdateSource(repository)
	if e != nil {
		return Profile{}, e
	}
	name := strings.ToLower(source.Repository)
	hash := sha256.Sum256([]byte("OShell GitHub update token\n" + name))
	p := NewProfile()
	p.ID = fmt.Sprintf("%x-%x-%x-%x-%x", hash[:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
	p.Name = "GitHub updates"
	p.Host = "api.github.com"
	p.Username = name
	return p, nil
}
func ReadUpdateToken(directory, repository string) (string, error) {
	p, e := updateCredential(repository)
	if e != nil {
		return "", e
	}
	dir := filepath.Join(directory, "update-auth")
	if _, e := os.Stat(dir); os.IsNotExist(e) {
		return "", nil
	}
	v, e := OpenVault(dir)
	if e != nil {
		return "", fmt.Errorf("更新令牌存储不可用：%w", e)
	}
	defer v.Lock()
	return v.Get(p)
}
func SaveUpdateToken(directory, repository, token string) error {
	p, e := updateCredential(repository)
	if e != nil {
		return e
	}
	token = strings.TrimSpace(token)
	if len(token) > 512 || strings.IndexFunc(token, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
	}) >= 0 {
		return fmt.Errorf("访问令牌格式无效")
	}
	dir := filepath.Join(directory, "update-auth")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	v, e := OpenVault(dir)
	if e != nil {
		return e
	}
	defer v.Lock()
	if token == "" {
		return v.Delete([]string{p.ID})
	}
	return v.Put(p, token)
}
