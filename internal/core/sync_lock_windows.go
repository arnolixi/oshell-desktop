//go:build windows

package core

import (
	"context"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"time"
)

func lockSyncDirectory(ctx context.Context, dir string) (func(), error) {
	return lockNamedDirectory(ctx, dir, ".oshell-sync.lock")
}

func lockNamedDirectory(ctx context.Context, dir, name string) (func(), error) {
	file, e := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, safeSyncFailure("directory")
	}
	var overlapped windows.Overlapped
	for {
		e = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
		if e == nil {
			return func() { windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped); file.Close() }, nil
		}
		if e != windows.ERROR_LOCK_VIOLATION {
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
