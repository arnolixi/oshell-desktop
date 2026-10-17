// Package bundled supplies the resources embedded by the Windows single-file
// build. Ordinary development builds continue to use external resources.
package bundled

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Set by the generated, platform-specific embed file during release builds.
var payload []byte

func Enabled() bool { return len(payload) != 0 }

var resources struct {
	sync.Once
	dir string
	err error
}

// Path verifies and materializes resources once per process, before any DLL
// is loaded. Each payload has its own cache, so upgrades do not replace a DLL
// still loaded by another instance.
func Path(name string) (string, error) {
	if !Enabled() || !validName(name) {
		return "", fmt.Errorf("内嵌资源不存在：%s", name)
	}
	resources.Do(func() {
		cache, err := os.UserCacheDir()
		if err != nil {
			resources.err = err
			return
		}
		resources.dir = filepath.Join(cache, "OShell Desktop", "runtime", fmt.Sprintf("%x", sha256.Sum256(payload)))
		resources.err = extract(payload, resources.dir)
	})
	if resources.err != nil {
		return "", fmt.Errorf("无法释放内嵌资源：%w", resources.err)
	}
	file := filepath.Join(resources.dir, filepath.FromSlash(name))
	info, err := os.Lstat(file)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("内嵌资源不是普通文件：%s", name)
	}
	return file, nil
}

func validName(name string) bool {
	return name != "" && name != "." && name != ".." && path.Clean(name) == name &&
		!strings.HasPrefix(name, "../") && !strings.ContainsAny(name, "\\:\x00") && !path.IsAbs(name)
}

func extract(data []byte, dir string) error {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	var total uint64
	seen := map[string]bool{}
	for _, file := range z.File {
		name := strings.ToLower(file.Name)
		if !validName(file.Name) || seen[name] || !file.Mode().IsRegular() || file.UncompressedSize64 > 32<<20 {
			return fmt.Errorf("内嵌资源路径或大小无效：%s", file.Name)
		}
		seen[name] = true
		total += file.UncompressedSize64
		if total > 128<<20 {
			return fmt.Errorf("内嵌资源超过大小限制")
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	for _, file := range z.File {
		reader, err := file.Open()
		if err != nil {
			return err
		}
		content, err := io.ReadAll(io.LimitReader(reader, (32<<20)+1))
		reader.Close()
		if err != nil {
			return err
		}
		if uint64(len(content)) != file.UncompressedSize64 {
			return fmt.Errorf("内嵌资源长度无效：%s", file.Name)
		}
		if err := publish(dir, file.Name, content); err != nil {
			return err
		}
	}
	return nil
}

func matches(file string, want []byte) bool {
	info, err := os.Lstat(file)
	if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(want)) {
		return false
	}
	got, err := os.ReadFile(file)
	return err == nil && sha256.Sum256(got) == sha256.Sum256(want)
}

func publish(root, name string, content []byte) error {
	directory := root
	// Reject redirected directories, including a redirected cache root.
	for _, part := range append([]string{""}, strings.Split(path.Dir(name), "/")...) {
		if part != "" && part != "." {
			directory = filepath.Join(directory, part)
		}
		if err := os.Mkdir(directory, 0700); err != nil && !os.IsExist(err) {
			return err
		}
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("资源缓存目录无效：%s", directory)
		}
	}
	target := filepath.Join(root, filepath.FromSlash(name))
	if matches(target, content) {
		return nil
	}
	if info, err := os.Lstat(target); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("资源缓存文件无效：%s", name)
	}
	tmp, err := os.CreateTemp(directory, ".resource-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(content)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 50; attempt++ {
		// Recheck before replacing: another process may already have published
		// and loaded this DLL. Windows can briefly deny both rename and read
		// while competing publishers or scanners have the destination open.
		if matches(target, content) {
			return nil
		}
		err = os.Rename(tmp.Name(), target)
		if err == nil {
			return nil
		}
		if attempt < 49 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	return err
}
