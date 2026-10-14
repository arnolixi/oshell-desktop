//go:build !windows

package core

import (
	"golang.org/x/sys/unix"
	"os"
)

func openImportFile(path string) (*os.File, error) {
	fd, e := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(fd), path), nil
}
