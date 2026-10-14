//go:build darwin

package core

import (
	"context"
	"crypto/sha256"
	"debug/macho"
	"encoding/json"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type PreparedUpdate struct {
	originalFingerprint  [32]byte
	mu                   sync.Mutex
	download             *VerifiedDownload
	target, work, bundle string
	original             os.FileInfo
	fingerprint          [32]byte
	verify               func(context.Context, string, string) error
	committed            bool
}

func (p *PreparedUpdate) Discard() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed {
		return fmt.Errorf("已安装更新的备份不能作为未安装暂存删除")
	}
	return os.RemoveAll(p.work)
}
func verifyNativeUpdateBundle(ctx context.Context, bundle, target string) error {
	info, e := ReadAppBundleInfo(bundle)
	if e != nil {
		return e
	}
	binary := filepath.Join(bundle, "Contents/MacOS", info.Executable)
	stat, e := os.Lstat(binary)
	if e != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("更新应用执行文件无效")
	}
	want := macho.CpuArm64
	if target == "darwin-amd64" {
		want = macho.CpuAmd64
	} else if target != "darwin-arm64" {
		return fmt.Errorf("不是受支持的 macOS 架构")
	}
	supported := false
	if fat, e := macho.OpenFat(binary); e == nil {
		for _, arch := range fat.Arches {
			supported = supported || arch.Cpu == want
		}
		fat.Close()
	} else if thin, e := macho.Open(binary); e == nil {
		supported = thin.Cpu == want
		thin.Close()
	}
	if !supported {
		return fmt.Errorf("更新程序架构与当前目标不匹配")
	}
	if e = exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", bundle).Run(); e != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("更新应用代码签名无效")
	}
	return nil
}
func bundleFingerprint(ctx context.Context, bundle string) ([32]byte, error) {
	hash := sha256.New()
	e := filepath.WalkDir(bundle, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		relative, e := filepath.Rel(bundle, path)
		if e != nil {
			return e
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		fmt.Fprintf(hash, "%s\x00%d\x00%d\x00", relative, info.Mode(), info.Size())
		if info.Mode()&os.ModeSymlink != 0 {
			target, e := os.Readlink(path)
			if e != nil {
				return e
			}
			hash.Write([]byte(target))
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("更新应用含特殊文件")
		}
		file, e := openImportFile(path)
		if e != nil {
			return e
		}
		_, e = io.Copy(hash, updateContextReader{ctx, file})
		file.Close()
		return e
	})
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result, e
}
func (d *VerifiedDownload) PrepareMacOS(ctx context.Context, target string) (*PreparedUpdate, error) {
	return d.prepareMacOS(ctx, target, verifyNativeUpdateBundle)
}
func (d *VerifiedDownload) prepareMacOS(ctx context.Context, target string, verify func(context.Context, string, string) error) (result *PreparedUpdate, err error) {
	if d == nil || d.update == nil {
		return nil, fmt.Errorf("更新尚未验证")
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	original, err := os.Lstat(target)
	if err != nil || !original.IsDir() || original.Mode()&os.ModeSymlink != 0 || !strings.HasSuffix(target, ".app") {
		return nil, fmt.Errorf("当前应用路径无效")
	}
	current, err := ReadAppBundleInfo(target)
	if err != nil {
		return nil, err
	}
	if current.Identifier != d.update.applicationID {
		return nil, fmt.Errorf("不能替换其他应用")
	}
	comparison, e := CompareUpdateVersions(d.update.version, current.Version)
	if e != nil || comparison < 0 {
		return nil, fmt.Errorf("更新版本不能回退")
	}
	if comparison == 0 {
		old, e := strconv.ParseUint(current.Build, 10, 64)
		next, _ := strconv.ParseUint(d.update.build, 10, 64)
		if e != nil || next <= old {
			return nil, fmt.Errorf("更新构建号不能回退")
		}
	}
	originalFingerprint, err := bundleFingerprint(ctx, target)
	if err != nil {
		return nil, err
	}
	archive, err := d.verifiedArchive(ctx)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	work, err := os.MkdirTemp(filepath.Dir(target), ".oshell-update-")
	if err != nil {
		return nil, err
	}
	defer func() {
		if result == nil {
			os.RemoveAll(work)
		}
	}()
	bundle, err := extractUpdateBundle(ctx, archive, work)
	if err != nil {
		return nil, err
	}
	incoming, err := ReadAppBundleInfo(bundle)
	if err != nil {
		return nil, err
	}
	if incoming.Identifier != d.update.applicationID || incoming.Version != d.update.version || incoming.Build != d.update.build || incoming.MinimumOS != d.update.archive.MinimumOS {
		return nil, fmt.Errorf("安装包应用标识、版本、构建号或最低系统不匹配")
	}
	if err = verify(ctx, bundle, d.update.target); err != nil {
		return nil, err
	}
	fingerprint, err := bundleFingerprint(ctx, bundle)
	if err != nil {
		return nil, err
	}
	return &PreparedUpdate{download: d, target: target, work: work, bundle: bundle, original: original, originalFingerprint: originalFingerprint, fingerprint: fingerprint, verify: verify}, nil
}

type InstalledUpdate struct {
	receipt               string
	mu                    sync.Mutex
	finalized, rolledBack bool
	target, backup, work  string
	fingerprint           [32]byte
}

// Commit performs one filesystem-atomic exchange on the same volume. The old
// bundle is retained for rollback/finalization after successful relaunch.
func (p *PreparedUpdate) Commit(ctx context.Context) (*InstalledUpdate, error) {
	return p.commit(ctx, "")
}
func (p *PreparedUpdate) CommitWithReceipt(ctx context.Context, stateDirectory string) (*InstalledUpdate, error) {
	if e := os.MkdirAll(stateDirectory, 0700); e != nil {
		return nil, e
	}
	return p.commit(ctx, filepath.Join(stateDirectory, "update-installation.json"))
}
func (p *PreparedUpdate) commit(ctx context.Context, receiptPath string) (*InstalledUpdate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed {
		return nil, fmt.Errorf("此更新已安装")
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	current, e := os.Lstat(p.target)
	if e != nil || !os.SameFile(p.original, current) {
		return nil, fmt.Errorf("当前应用已被其他操作替换")
	}
	originalFingerprint, e := bundleFingerprint(ctx, p.target)
	if e != nil || originalFingerprint != p.originalFingerprint {
		return nil, fmt.Errorf("当前应用内容已变化，请重新检查更新")
	}
	fingerprint, e := bundleFingerprint(ctx, p.bundle)
	if e != nil || fingerprint != p.fingerprint {
		return nil, fmt.Errorf("暂存应用已变化，请重新下载")
	}
	if e = p.verify(ctx, p.bundle, p.download.update.target); e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	if receiptPath != "" {
		if _, e := os.Lstat(receiptPath); e == nil {
			return nil, fmt.Errorf("已有更新恢复记录，请先重启完成恢复")
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		record := updateInstallationRecord{Version: 1, Target: p.target, Work: p.work, Backup: p.bundle, ApplicationID: p.download.update.applicationID, NewVersion: p.download.update.version, NewBuild: p.download.update.build, Original: p.originalFingerprint, Installed: p.fingerprint}
		data, e := json.Marshal(record)
		if e != nil {
			return nil, e
		}
		if e = AtomicWrite(filepath.Join(p.work, "installation.json"), data); e != nil {
			return nil, e
		}
		if e = AtomicWrite(receiptPath, data); e != nil {
			return nil, e
		}
		if directory, e := os.Open(filepath.Dir(receiptPath)); e == nil {
			syncErr := directory.Sync()
			directory.Close()
			if syncErr != nil {
				os.Remove(receiptPath)
				return nil, syncErr
			}
		} else {
			os.Remove(receiptPath)
			return nil, e
		}
		if e = ctx.Err(); e != nil {
			os.Remove(receiptPath)
			return nil, e
		}
	}
	if e = unix.RenameatxNp(unix.AT_FDCWD, p.bundle, unix.AT_FDCWD, p.target, unix.RENAME_SWAP); e != nil {
		if receiptPath != "" {
			os.Remove(receiptPath)
		}
		return nil, fmt.Errorf("无法原子替换应用：%w", e)
	}
	p.committed = true
	return &InstalledUpdate{target: p.target, backup: p.bundle, work: p.work, fingerprint: p.fingerprint, receipt: receiptPath}, nil
}
func (i *InstalledUpdate) Rollback(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.finalized || i.rolledBack {
		return fmt.Errorf("此安装已完成清理或回滚")
	}
	current, e := bundleFingerprint(ctx, i.target)
	if e != nil || current != i.fingerprint {
		return fmt.Errorf("当前应用已变化，未回滚")
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	if e := unix.RenameatxNp(unix.AT_FDCWD, i.backup, unix.AT_FDCWD, i.target, unix.RENAME_SWAP); e != nil {
		return e
	}
	i.rolledBack = true
	if i.receipt != "" {
		return os.Remove(i.receipt)
	}
	return nil
}
func (i *InstalledUpdate) Finalize() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.finalized {
		return nil
	}
	if e := os.RemoveAll(i.work); e != nil {
		return e
	}
	i.finalized = true
	if i.receipt != "" {
		if e := os.Remove(i.receipt); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	return nil
}
