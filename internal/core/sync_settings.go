package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SyncOptions struct {
	RequireExisting bool   `json:"requireExisting,omitempty"`
	Version         int    `json:"version"`
	Enabled         bool   `json:"enabled"`
	Backend         string `json:"backend"`
	Address         string `json:"address"`
	Username        string `json:"username,omitempty"`
}

var ErrSyncSettingsLocked = errors.New("WebDAV 设置已加密，请先解锁主密码")

func LoadSyncOptions(dir string, master ...[]byte) (SyncOptions, error) {
	options := SyncOptions{Version: 1, Backend: "webdav"}
	data, e := ReadBounded(filepath.Join(dir, "sync-settings.json"))
	if os.IsNotExist(e) {
		return options, nil
	}
	if e != nil {
		return options, e
	}
	if IsSharedVault(data) {
		if len(master) == 0 || len(master[0]) == 0 {
			return options, ErrSyncSettingsLocked
		}
		plain, err := OpenShared(data, master[0])
		if err != nil {
			return options, err
		}
		defer clear(plain)
		data = plain
	}
	if e = json.Unmarshal(data, &options); e != nil || options.Version != 1 {
		return options, fmt.Errorf("同步设置格式无效")
	}
	return options, nil
}
func (o SyncOptions) Target() string {
	return o.Backend + "\n" + strings.TrimSpace(o.Address) + "\n" + o.Username
}
func (o SyncOptions) CredentialProfile() Profile {
	p := NewProfile()
	p.ID = "81E40DA6-0A65-4F9B-BA44-7C794F10C754"
	p.Kind = "local"
	p.Host = o.Address
	p.Username = o.Username
	p.Name = "WebDAV sync service"
	return p
}
func (o SyncOptions) Transport(password, localDirectory string) (SyncTransport, error) {
	switch o.Backend {
	case "webdav":
		return NewWebDAVTransport(o.Address, o.Username, password, false)
	case "directory":
		path, e := filepath.Abs(strings.TrimSpace(o.Address))
		if e != nil {
			return nil, e
		}
		path, e = filepath.EvalSymlinks(path)
		if e != nil {
			return nil, fmt.Errorf("同步目录不可用")
		}
		local, e := filepath.Abs(localDirectory)
		if e != nil {
			return nil, e
		}
		local, e = filepath.EvalSymlinks(local)
		if e != nil {
			return nil, e
		}
		if syncDirectoriesOverlap(path, local) {
			return nil, fmt.Errorf("同步目录必须与本机配置目录相互独立")
		}
		d := DirectoryTransport{Directory: path}
		if e = d.check(); e != nil {
			return nil, e
		}
		return d, nil
	default:
		return nil, fmt.Errorf("请选择同步方式")
	}
}
func SaveSyncOptions(dir string, o SyncOptions, master ...[]byte) error {
	release, err := lockStateDirectory(context.Background(), dir, false)
	if err != nil {
		return err
	}
	defer release()
	return saveSyncOptionsLocked(dir, o, master...)
}
func saveSyncOptionsLocked(dir string, o SyncOptions, master ...[]byte) error {
	if o.Version != 1 {
		return fmt.Errorf("同步设置版本无效")
	}
	data, e := json.MarshalIndent(o, "", "  ")
	if e != nil {
		return e
	}
	previous, err := LoadSyncOptions(dir, master...)
	if err != nil {
		return err
	}
	if o.Backend == "webdav" {
		if o.Enabled {
			if len(master) == 0 || len(master[0]) == 0 {
				return ErrSyncSettingsLocked
			}
			plain := data
			data, e = SealShared(plain, master[0])
			clear(plain)
			if e != nil {
				return e
			}
		} else {
			// Native OShell removes disabled WebDAV login settings.
			o.Address = ""
			o.Username = ""
			data, e = json.MarshalIndent(o, "", "  ")
			if e != nil {
				return e
			}
		}
	}
	oldPath, newPath := "", ""
	if previous.Enabled && (!o.Enabled || previous.Target() != o.Target()) {
		oldPath = (SharedReplicaStore{Directory: dir, Target: previous.Target()}).path()
		newPath = oldPath + ".archived-" + NewID()
		if err = os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		if os.IsNotExist(err) {
			oldPath = ""
		}
	}
	if err = AtomicWrite(filepath.Join(dir, "sync-settings.json"), data); err != nil {
		if oldPath != "" {
			_ = os.Rename(newPath, oldPath)
		}
		return err
	}
	return nil
}

// SyncOptionsStatus exposes only enabled/backend for locked startup and guards.
func SyncOptionsStatus(dir string) (SyncOptions, error) {
	options, err := LoadSyncOptions(dir)
	if errors.Is(err, ErrSyncSettingsLocked) {
		return SyncOptions{Version: 1, Enabled: true, Backend: "webdav"}, nil
	}
	return options, err
}

// Upgrade under the publication lock, reading the latest version rather than
// republishing a snapshot captured before another process saved its settings.
func LoadAndProtectSyncOptions(dir string, master []byte) (SyncOptions, error) {
	release, err := lockStateDirectory(context.Background(), dir, false)
	if err != nil {
		return SyncOptions{}, err
	}
	defer release()
	options, err := LoadSyncOptions(dir, master)
	if err != nil {
		return options, err
	}
	if len(master) > 0 && options.Enabled && options.Backend == "webdav" {
		raw, err := ReadBounded(filepath.Join(dir, "sync-settings.json"))
		if err != nil {
			return options, err
		}
		if !IsSharedVault(raw) {
			err = saveSyncOptionsLocked(dir, options, master)
			return options, err
		}
	}
	return options, nil
}
