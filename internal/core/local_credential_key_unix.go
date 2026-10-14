//go:build !windows

package core

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

func validateLocalKeyFile(file *os.File, limit int64, repair bool) error {
	var stat unix.Stat_t
	if e := unix.Fstat(int(file.Fd()), &stat); e != nil {
		return e
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Uid != uint32(os.Geteuid()) || stat.Size > limit || stat.Size < 0 || stat.Mode&0077 != 0 && !repair {
		return fmt.Errorf("本机密钥需为当前用户拥有且仅当前用户可读写的普通文件")
	}
	return nil
}
func lockLocalKey(dir string) (func(), error) {
	fd, e := unix.Open(filepath.Join(dir, ".local-credential-key.lock"), unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0600)
	if e != nil {
		return nil, e
	}
	file := os.NewFile(uintptr(fd), "local key lock")
	if e = validateLocalKeyFile(file, 4096, false); e != nil {
		file.Close()
		return nil, e
	}
	if e = unix.Flock(fd, unix.LOCK_EX); e != nil {
		file.Close()
		return nil, e
	}
	return func() { unix.Flock(fd, unix.LOCK_UN); file.Close() }, nil
}
func syncLocalKeyDirectory(dir string) error {
	file, e := os.Open(dir)
	if e != nil {
		return e
	}
	defer file.Close()
	return file.Sync()
}

func publishLocalCredentialKey(source, destination string) error { return os.Link(source, destination) }
