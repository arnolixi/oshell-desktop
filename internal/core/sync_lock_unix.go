//go:build !windows

package core

import (
	"context"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"time"
)

func lockSyncDirectory(ctx context.Context, dir string) (func(), error) {
	return lockNamedDirectory(ctx, dir, ".oshell-sync.lock")
}

func lockNamedDirectory(ctx context.Context, dir, name string) (func(), error) {
	fd, e := unix.Open(filepath.Join(dir, name), unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0600)
	if e != nil {
		return nil, safeSyncFailure("directory")
	}
	file := os.NewFile(uintptr(fd), "sync lock")
	info, e := file.Stat()
	if e != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, safeSyncFailure("directory")
	}
	for {
		if e = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); e == nil {
			return func() { unix.Flock(fd, unix.LOCK_UN); file.Close() }, nil
		}
		if e != unix.EWOULDBLOCK && e != unix.EAGAIN {
			file.Close()
			return nil, safeSyncFailure("directory")
		}
		select {
		case <-ctx.Done():
			file.Close()
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
