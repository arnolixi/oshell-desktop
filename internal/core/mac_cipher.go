// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/pbkdf2"
)

// LegacyPassword is the OShell 0.2.64–0.2.67 EncryptedPassword wire format.
type LegacyPassword struct {
	LocalKeyID string      `json:"localKeyID,omitempty"`
	Version    int         `json:"version"`
	Algorithm  string      `json:"algorithm"`
	KDF        string      `json:"kdf"`
	Iterations int         `json:"iterations"`
	Salt       string      `json:"salt"`
	Ciphertext string      `json:"ciphertext"`
	Identity   SSHIdentity `json:"identity"`
}

func macPasswordBinding(p Profile, e LegacyPassword) string {
	return "OShell-password-v1\x00" + strings.ToUpper(p.ID) + "\x00" + p.Host + "\x00" + strconv.Itoa(p.Port) + "\x00" + p.Username + "\x00" + e.Identity.Host + "\x00" + e.Identity.User + "\x00" + strconv.Itoa(e.Identity.Port)
}
func EncryptMacPassword(password string, master []byte, p Profile, identity SSHIdentity) (LegacyPassword, error) {
	e := LegacyPassword{Version: 1, Algorithm: "AES-256-GCM", KDF: "PBKDF2-HMAC-SHA256", Iterations: 600000, Identity: identity}
	if len(master) == 0 || password == "" || len(password) > 4096 || strings.ContainsAny(password, "\x00\r\n") {
		return e, fmt.Errorf("密码或主密码格式无效")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return e, err
	}
	e.Salt = base64.StdEncoding.EncodeToString(salt)
	key := pbkdf2.Key(master, salt, e.Iterations, 32, sha256.New)
	defer clear(key)
	data, err := seal(key, []byte(password), macPasswordBinding(p, e))
	if err != nil {
		return e, err
	}
	e.Ciphertext = base64.StdEncoding.EncodeToString(data)
	return e, nil
}
func DecryptMacPassword(e LegacyPassword, master []byte, p Profile) (string, error) {
	if e.LocalKeyID != "" {
		return "", fmt.Errorf("此密码使用 macOS 本机密钥，请在原版导出时转换为导出密码保护")
	}
	salt, err := base64.StdEncoding.DecodeString(e.Salt)
	data, decodeErr := base64.StdEncoding.DecodeString(e.Ciphertext)
	if err != nil || decodeErr != nil || e.Version != 1 || e.Algorithm != "AES-256-GCM" || e.KDF != "PBKDF2-HMAC-SHA256" || len(master) == 0 || len(salt) != 16 || e.Iterations < 600000 || e.Iterations > 2000000 || len(data) < 28 || len(data) > 8192 {
		return "", fmt.Errorf("加密密码格式无效")
	}
	key := pbkdf2.Key(master, salt, e.Iterations, 32, sha256.New)
	defer clear(key)
	plain, err := unseal(key, data, macPasswordBinding(p, e))
	if err != nil || !utf8.Valid(plain) {
		clear(plain)
		return "", fmt.Errorf("主密码不正确，或密码/连接身份已被修改")
	}
	defer clear(plain)
	return string(plain), nil
}
