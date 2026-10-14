package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PendingSyncDirectory struct {
	Version int    `json:"version"`
	Path    string `json:"path"`
	Mode    string `json:"mode"`
}

func syncDirectoriesOverlap(a, b string) bool {
	within := func(child, parent string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	return within(a, b) || within(b, a)
}
func LoadPendingSyncDirectory(local string) (*PendingSyncDirectory, error) {
	data, err := ReadBounded(filepath.Join(local, "pending-sync-directory.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pending PendingSyncDirectory
	if err = json.Unmarshal(data, &pending); err != nil || pending.Version != 1 || !filepath.IsAbs(pending.Path) || (pending.Mode != "create" && pending.Mode != "existing") {
		return nil, fmt.Errorf("待生效目录设置无效")
	}
	return &pending, nil
}
func CancelPendingSyncDirectory(local string) error {
	err := os.Remove(filepath.Join(local, "pending-sync-directory.json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func validateSyncDirectoryChoice(local string, pending PendingSyncDirectory) (SyncOptions, error) {
	options := SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: pending.Path, RequireExisting: pending.Mode == "existing"}
	transport, err := options.Transport("", local)
	if err != nil {
		return options, err
	}
	if pending.Mode == "create" {
		entries, err := os.ReadDir(pending.Path)
		if err != nil {
			return options, err
		}
		for _, entry := range entries {
			if entry.Name() != ".DS_Store" {
				return options, fmt.Errorf("新同步目录必须为空；已有数据请使用连接已有目录")
			}
		}
	} else {
		object, err := transport.Fetch(context.Background())
		if err != nil {
			return options, err
		}
		if object == nil {
			return options, fmt.Errorf("目录中没有 configuration.json 加密同步数据")
		}
	}
	return options, nil
}
func ValidateSyncDirectorySelection(local, target, mode string, master []byte) (*PendingSyncDirectory, error) {
	if len(master) == 0 {
		return nil, fmt.Errorf("请先设置并解锁主密码")
	}
	if mode != "create" && mode != "existing" {
		return nil, fmt.Errorf("目录操作无效")
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	pending := PendingSyncDirectory{Version: 1, Path: absolute, Mode: mode}
	options, err := validateSyncDirectoryChoice(local, pending)
	if err != nil {
		return nil, err
	}
	if mode == "existing" {
		transport, _ := options.Transport("", local)
		object, err := transport.Fetch(context.Background())
		if err != nil {
			return nil, err
		}
		if object == nil {
			return nil, fmt.Errorf("同步数据已消失")
		}
		if _, err = DecodeSharedDocument(object.Data, master); err != nil {
			return nil, err
		}
	}
	active, err := LoadSyncOptions(local, master)
	if err != nil {
		return nil, err
	}
	if active.Enabled && active.Target() == options.Target() {
		return nil, fmt.Errorf("该目录已经用于同步")
	}
	return &pending, nil
}

func ScheduleSyncDirectory(local, target, mode string, master []byte) error {
	pending, err := ValidateSyncDirectorySelection(local, target, mode, master)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(pending, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(filepath.Join(local, "pending-sync-directory.json"), data)
}

// Activation changes only the sync destination. Local configuration, credentials,
// keys and known_hosts remain local; encrypted merge runs after vault unlock.
func ActivatePendingSyncDirectory(local string) error {
	pending, err := LoadPendingSyncDirectory(local)
	if err != nil || pending == nil {
		return err
	}
	active, err := LoadSyncOptions(local)
	if err != nil {
		return err
	}
	desired := SyncOptions{Version: 1, Enabled: true, Backend: "directory", Address: pending.Path, RequireExisting: pending.Mode == "existing"}
	if active == desired {
		return CancelPendingSyncDirectory(local)
	}
	options, err := validateSyncDirectoryChoice(local, *pending)
	if err != nil {
		return err
	}
	if err = SaveSyncOptions(local, options); err != nil {
		return err
	}
	return CancelPendingSyncDirectory(local)
}
