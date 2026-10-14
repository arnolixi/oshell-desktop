package core

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalCredentialKey matches OShell 0.2.67's app-managed key document. It is
// intentionally not a Keychain item and never invokes a system password UI.
type LocalCredentialKey struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

func (k LocalCredentialKey) String() string   { return "<local credential key>" }
func (k LocalCredentialKey) GoString() string { return k.String() }
func LoadOShellLocalCredentialKey(directory, expectedID string, repairPermissions bool) (LocalCredentialKey, error) {
	var key LocalCredentialKey
	file, e := openImportFile(filepath.Join(directory, "local-credential-key.json"))
	if e != nil {
		return key, fmt.Errorf("无法读取 OShell 本机密钥，请恢复原密钥或重新保存密码")
	}
	defer file.Close()
	if e = validateLocalKeyFile(file, 4096, repairPermissions); e != nil {
		return key, e
	}
	data, e := io.ReadAll(io.LimitReader(file, 4097))
	if e != nil {
		return key, e
	}
	defer clear(data)
	if len(data) > 4096 || json.Unmarshal(data, &key) != nil || !uuidText.MatchString(key.ID) {
		return LocalCredentialKey{}, fmt.Errorf("本机密钥文件损坏，原文件已保留")
	}
	decoded, e := base64.StdEncoding.DecodeString(key.Secret)
	defer clear(decoded)
	if e != nil || len(decoded) != 32 {
		return LocalCredentialKey{}, fmt.Errorf("本机密钥文件损坏，原文件已保留")
	}
	if expectedID != "" && !strings.EqualFold(expectedID, key.ID) {
		return LocalCredentialKey{}, fmt.Errorf("密码与本机密钥 ID 不匹配")
	}
	if repairPermissions {
		if e = file.Chmod(0600); e != nil {
			return LocalCredentialKey{}, fmt.Errorf("无法修复本机密钥权限")
		}
	}
	return key, nil
}
func loadLocalVaultKey(dir string) ([]byte, error) {
	file, e := openImportFile(filepath.Join(dir, "credential-key"))
	if e != nil {
		return nil, e
	}
	defer file.Close()
	if e = validateLocalKeyFile(file, 32, false); e != nil {
		return nil, e
	}
	key, e := io.ReadAll(io.LimitReader(file, 33))
	if e != nil || len(key) != 32 {
		clear(key)
		return nil, fmt.Errorf("本机凭据密钥无效，原文件已保留")
	}
	return key, nil
}
func localVaultKey(dir string) ([]byte, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	unlock, e := lockLocalKey(dir)
	if e != nil {
		return nil, e
	}
	defer unlock()
	if key, e := loadLocalVaultKey(dir); e == nil {
		return key, nil
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	key := make([]byte, 32)
	if _, e = rand.Read(key); e != nil {
		return nil, e
	}
	file, e := os.CreateTemp(dir, ".local-key-")
	if e != nil {
		clear(key)
		return nil, e
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, e = file.Write(key); e == nil {
		e = file.Sync()
	}
	closeErr := file.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		clear(key)
		return nil, e
	}
	// Atomic no-replace publication: readers never observe a partial key.
	if e = publishLocalCredentialKey(temporary, filepath.Join(dir, "credential-key")); e != nil {
		clear(key)
		if os.IsExist(e) {
			return loadLocalVaultKey(dir)
		}
		return nil, e
	}
	if e = syncLocalKeyDirectory(dir); e != nil {
		clear(key)
		return nil, e
	}
	return key, nil
}

// DecryptMacLocalPassword is the explicit migration path for a native local-key
// envelope. Ordinary portable imports still reject local-key references.
func DecryptMacLocalPassword(envelope LegacyPassword, key LocalCredentialKey, profile Profile) (string, error) {
	decoded, e := base64.StdEncoding.DecodeString(key.Secret)
	defer clear(decoded)
	if e != nil || len(decoded) != 32 || !uuidText.MatchString(key.ID) || envelope.LocalKeyID == "" || !strings.EqualFold(envelope.LocalKeyID, key.ID) {
		return "", fmt.Errorf("密码与本机密钥 ID 不匹配")
	}
	master := []byte(key.Secret)
	defer clear(master)
	// The source cipher binds profile/resolved identity, while localKeyID chooses
	// the key store. It is not part of the AES-GCM additional authenticated data.
	envelope.LocalKeyID = ""
	password, e := DecryptMacPassword(envelope, master, profile)
	if e != nil {
		return "", fmt.Errorf("本机密钥与密码或连接信息不匹配")
	}
	return password, nil
}
