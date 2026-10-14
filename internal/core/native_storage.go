package core

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// NativeStorageSnapshot identifies the active encrypted database of a native
// OShell installation. Reading never activates pending settings or writes source data.
type NativeStorageSnapshot struct {
	WebDAV           *SyncOptions
	webDAVData       []byte
	LocalCredentials bool
	PlainLocal       bool
	Directory        string
	SyncDirectory    string
	LegacyExternal   bool
	Document         SharedDocument
}

func ReadNativeStorage(directory string, master []byte) (NativeStorageSnapshot, error) {
	result := NativeStorageSnapshot{}
	base, err := filepath.Abs(directory)
	if err != nil {
		return result, err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return result, err
	}
	result.Directory = base
	var state struct {
		Current        *string `json:"current"`
		ReplicaVersion *int    `json:"replicaVersion"`
	}
	stateData, err := ReadBounded(filepath.Join(base, "storage-location.json"))
	if err != nil && !os.IsNotExist(err) {
		return result, err
	}
	if err == nil {
		if err = json.Unmarshal(stateData, &state); err != nil {
			return result, fmt.Errorf("原版存储位置设置无效：%w", err)
		}
		if state.Current != nil {
			if !filepath.IsAbs(*state.Current) {
				return result, fmt.Errorf("原版同步目录不是绝对路径")
			}
			result.SyncDirectory = filepath.Clean(*state.Current)
			_, localErr := os.Stat(filepath.Join(base, "configuration.json"))
			if localErr != nil && !os.IsNotExist(localErr) {
				return result, localErr
			}
			if state.ReplicaVersion == nil || *state.ReplicaVersion != 2 || os.IsNotExist(localErr) {
				result.Directory, err = filepath.EvalSymlinks(*state.Current)
				if err != nil {
					return result, err
				}
				result.LegacyExternal = true
				for _, name := range []string{"local-credential-key.json", "credential-key"} {
					if _, err = os.Lstat(filepath.Join(result.Directory, name)); err == nil {
						return result, fmt.Errorf("旧共享目录包含本机密钥，无法作为加密同步来源")
					} else if !os.IsNotExist(err) {
						return result, err
					}
				}
			}
		}
	}
	file := filepath.Join(result.Directory, "configuration.json")
	stream, err := openImportFile(file)
	if err != nil {
		return result, err
	}
	defer stream.Close()
	info, err := stream.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() {
		return result, fmt.Errorf("原版配置不是普通文件")
	}
	data, err := io.ReadAll(io.LimitReader(stream, MaxSharedVault+1))
	if err != nil {
		return result, err
	}
	defer clear(data)
	if len(data) > MaxSharedVault {
		return result, fmt.Errorf("原版配置超过允许大小")
	}
	var format struct {
		Format string `json:"format"`
	}
	if err = json.Unmarshal(data, &format); err != nil {
		return result, err
	}
	if format.Format == "OShell.shared-vault" {
		if result.LegacyExternal {
			result.Document, err = DecodeSharedDocument(data, master)
		} else {
			var plain []byte
			plain, err = OpenShared(data, master)
			if err == nil {
				result.Document, err = decodeNativeDocument(plain)
			}
			clear(plain)
			result.LocalCredentials = true
			if err == nil {
				err = result.validatePlainNative(master)
			}
		}
	} else {
		if result.LegacyExternal {
			return result, fmt.Errorf("旧共享目录必须包含完整加密配置")
		}
		result.PlainLocal = true
		result.Document, err = decodeNativeDocument(data)
		if err == nil {
			err = result.validatePlainNative(master)
		}
	}
	if err != nil {
		return result, fmt.Errorf("无法解密原版配置：%w", err)
	}
	davData, davErr := ReadBounded(filepath.Join(base, "webdav-settings.json"))
	if davErr != nil && !os.IsNotExist(davErr) {
		return result, davErr
	}
	if davErr == nil {
		options, _, decodeErr := decodeNativeWebDAV(davData, master)
		if decodeErr != nil {
			return result, fmt.Errorf("无法读取原版 WebDAV 设置：%w", decodeErr)
		}
		result.WebDAV = &options
		result.webDAVData = davData
	}
	return result, nil
}
