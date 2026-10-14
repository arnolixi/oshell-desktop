//go:build darwin

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type updateInstallationRecord struct {
	Version                                                   int
	Target, Work, Backup, ApplicationID, NewVersion, NewBuild string
	Original, Installed                                       [32]byte
}

// FinishUpdateRecovery runs in the newly launched app. It only removes a
// verified installer-owned staging directory next to this exact application.
// Ambiguous state preserves the record and backup for manual recovery.
func FinishUpdateRecovery(ctx context.Context, stateDirectory, target, applicationID string) error {
	recordPath := filepath.Join(stateDirectory, "update-installation.json")
	file, e := openImportFile(recordPath)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	data, e := io.ReadAll(io.LimitReader(file, 16385))
	file.Close()
	if e != nil || len(data) > 16384 {
		return fmt.Errorf("更新恢复记录无效")
	}
	var record updateInstallationRecord
	if json.Unmarshal(data, &record) != nil || record.Version != 1 {
		return fmt.Errorf("更新恢复记录无效")
	}
	target, e = filepath.Abs(target)
	if e != nil {
		return e
	}
	if target != record.Target || record.ApplicationID != applicationID || filepath.Dir(record.Work) != filepath.Dir(target) || !strings.HasPrefix(filepath.Base(record.Work), ".oshell-update-") || filepath.Dir(record.Backup) != record.Work || !strings.HasSuffix(record.Backup, ".app") {
		return fmt.Errorf("更新恢复路径不匹配，已保留备份")
	}
	info, e := ReadAppBundleInfo(target)
	if e != nil || info.Identifier != applicationID {
		return fmt.Errorf("当前应用身份不匹配，已保留备份")
	}
	fingerprint, e := bundleFingerprint(ctx, target)
	if e != nil {
		return e
	}
	if fingerprint != record.Original && (fingerprint != record.Installed || info.Version != record.NewVersion || info.Build != record.NewBuild) {
		return fmt.Errorf("当前应用内容已变化，已保留更新备份")
	}
	if stat, e := os.Lstat(record.Work); e == nil {
		if !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 || stat.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("更新暂存目录无效")
		}
		marker, e := openImportFile(filepath.Join(record.Work, "installation.json"))
		if e != nil {
			return fmt.Errorf("更新暂存缺少安装标记，已保留备份")
		}
		marked, e := io.ReadAll(io.LimitReader(marker, 16385))
		marker.Close()
		if e != nil || !bytes.Equal(marked, data) {
			return fmt.Errorf("更新暂存标记不匹配，已保留备份")
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	if e = os.RemoveAll(record.Work); e != nil {
		return e
	}
	return os.Remove(recordPath)
}
