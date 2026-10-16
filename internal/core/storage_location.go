package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const storageLocationFile = "storage-location.json"

type StorageLocation struct {
	Active  string `json:"active,omitempty"`
	Pending string `json:"pending,omitempty"`
}

const PortableMarker = "oshell-portable"

func portableStorageDirectory(goos, executable string) (string, bool, error) {
	if goos != "windows" {
		return "", false, nil
	}
	directory, err := filepath.Abs(filepath.Dir(executable))
	if err != nil {
		return "", false, err
	}
	info, err := os.Lstat(filepath.Join(directory, PortableMarker))
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !info.Mode().IsRegular() {
		return "", false, fmt.Errorf("便携模式标记必须为普通文件")
	}
	return directory, true, nil
}
func defaultStorageDirectory() (string, bool, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	if directory, portable, err := portableStorageDirectory(runtime.GOOS, executable); portable || err != nil {
		return directory, portable, err
	}
	dir := os.Getenv("OSHELL_DATA_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", false, err
		}
		dir = filepath.Join(base, "oshell-desktop")
	}
	dir, err = filepath.Abs(dir)
	return dir, false, err
}

func (s *Store) StorageBootstrapDirectory() string {
	if s.bootstrapDir != "" {
		return s.bootstrapDir
	}
	return s.Dir
}
func readStorageLocation(base string) (StorageLocation, error) {
	var location StorageLocation
	data, err := ReadBounded(filepath.Join(base, storageLocationFile))
	if os.IsNotExist(err) {
		return location, nil
	}
	if err != nil {
		return location, err
	}
	if err = json.Unmarshal(data, &location); err != nil {
		return location, fmt.Errorf("保存位置记录损坏：%w", err)
	}
	for _, p := range []string{location.Active, location.Pending} {
		if p != "" && !filepath.IsAbs(p) {
			return location, fmt.Errorf("保存位置必须为绝对路径")
		}
	}
	return location, nil
}
func (s *Store) StorageLocation() (StorageLocation, error) {
	if s.Portable {
		return StorageLocation{Active: s.Dir}, nil
	}
	location, err := readStorageLocation(s.StorageBootstrapDirectory())
	if location.Active == "" {
		location.Active = s.Dir
	}
	return location, err
}
func writeStorageLocation(base string, location StorageLocation) error {
	data, err := json.MarshalIndent(location, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(filepath.Join(base, storageLocationFile), data)
}

func storageTarget(source, target string) (string, error) {
	if !filepath.IsAbs(target) {
		return "", fmt.Errorf("请选择绝对路径")
	}
	target = filepath.Clean(target)
	src, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", err
	}
	// Resolve the parent, but reject a target symlink instead of following it.
	parent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil {
		return "", fmt.Errorf("目标的父目录必须存在：%w", err)
	}
	target = filepath.Join(parent, filepath.Base(target))
	if target == src {
		return target, nil
	}
	if err := validateStorageSeparation(src, target); err != nil {
		return "", err
	}
	if info, err := os.Lstat(target); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("目标必须是普通目录")
		}
		entries, e := os.ReadDir(target)
		if e != nil {
			return "", e
		}
		if len(entries) > 0 {
			return "", fmt.Errorf("目标目录非空，请选择新的或空目录，避免覆盖已有数据")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return target, nil
}
func (s *Store) ValidateStorageDirectory(target string) error {
	if s.Portable {
		if filepath.Clean(strings.TrimSpace(target)) == filepath.Clean(s.Dir) {
			return nil
		}
		return fmt.Errorf("便携模式的数据固定保存在 EXE 所在目录；请移动整个程序文件夹")
	}
	_, err := storageTarget(s.Dir, strings.TrimSpace(target))
	return err
}

// Schedule only a pointer. Actual copying happens at startup, including writes
// made after Apply. The active store and every open connection stay unchanged.
func (s *Store) ScheduleStorageDirectory(target string) error {
	if s.Portable {
		return s.ValidateStorageDirectory(target)
	}
	target, err := storageTarget(s.Dir, strings.TrimSpace(target))
	if err != nil {
		return err
	}
	base := s.StorageBootstrapDirectory()
	release, err := lockNamedDirectory(context.Background(), base, ".storage-location.lock")
	if err != nil {
		return err
	}
	defer release()
	location, err := readStorageLocation(base)
	if err != nil {
		return err
	}
	active := location.Active
	if active == "" {
		active = base
	}
	if filepath.Clean(active) != filepath.Clean(s.Dir) {
		return fmt.Errorf("保存位置已被其他实例修改，请重新启动")
	}
	current, err := filepath.EvalSymlinks(s.Dir)
	if err != nil {
		return err
	}
	if target == current {
		location.Pending = ""
	} else {
		location.Pending = target
	}
	return writeStorageLocation(base, location)
}

func activateStorageLocation(base string) (string, string, error) {
	release, err := lockNamedDirectory(context.Background(), base, ".storage-location.lock")
	if err != nil {
		return "", "", err
	}
	defer release()
	location, err := readStorageLocation(base)
	if err != nil {
		return "", "", err
	}
	source := location.Active
	if source == "" {
		source = base
	}
	if location.Active != "" {
		info, statErr := os.Stat(source)
		if statErr != nil {
			return "", "", fmt.Errorf("自定义数据目录不可用（未创建空配置）：%w", statErr)
		}
		if !info.IsDir() {
			return "", "", fmt.Errorf("自定义数据位置不是目录")
		}
	}
	if location.Pending == "" {
		return source, "", nil
	}
	fail := func(err error) (string, string, error) {
		return source, "保存位置未切换，继续使用原目录：" + err.Error(), nil
	}
	target, err := storageTarget(source, location.Pending)
	if err != nil {
		return fail(err)
	}
	if target == source {
		location.Pending = ""
		return source, "", writeStorageLocation(base, location)
	}
	if err = recoverTransaction(source); err != nil {
		return fail(err)
	}
	unlock, err := lockStateDirectory(context.Background(), source, false)
	if err != nil {
		return fail(err)
	}
	defer unlock()
	stage, err := os.MkdirTemp(filepath.Dir(target), ".oshell-migrate-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(stage)
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		rel, e := filepath.Rel(source, path)
		if e != nil {
			return e
		}
		if rel == storageLocationFile || strings.HasSuffix(entry.Name(), ".lock") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dest := filepath.Join(stage, rel)
		if entry.IsDir() {
			return os.Mkdir(dest, 0700)
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("无法迁移非普通文件：%s", rel)
		}
		in, e := openImportFile(path)
		if e != nil {
			return e
		}
		defer in.Close()
		out, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = io.Copy(out, in)
		if e == nil {
			e = out.Sync()
		}
		closeErr := out.Close()
		if e != nil {
			return e
		}
		return closeErr
	})
	if err != nil {
		return fail(err)
	}
	// Revalidate before publication: never merge into a populated directory.
	if _, err = storageTarget(source, target); err != nil {
		return fail(err)
	}
	// Remove only an empty directory; os.Remove refuses populated directories.
	// This also makes publication work on platforms that do not replace directories.
	if _, statErr := os.Lstat(target); statErr == nil {
		if err = os.Remove(target); err != nil {
			return fail(err)
		}
	} else if !os.IsNotExist(statErr) {
		return fail(statErr)
	}
	if err = os.Rename(stage, target); err != nil {
		return fail(err)
	}
	location.Active = target
	location.Pending = ""
	if err = writeStorageLocation(base, location); err != nil {
		return fail(err)
	}
	return target, "", nil
}
