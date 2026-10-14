package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Object mutexes (including conflict snapshot guards) precede this disk lock.
// Hold it across the final revision checks and complete publication. It is
// separate from the remote-sync lock, which callers may already own.
func lockStateDirectory(ctx context.Context, dir string, recovery bool) (func(), error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	release, err := lockNamedDirectory(ctx, dir, ".oshell-state.lock")
	if err != nil {
		return nil, fmt.Errorf("无法取得配置写入锁，请稍后重试: %w", err)
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	if !recovery {
		if _, err = os.Lstat(filepath.Join(dir, transactionFile)); err == nil {
			release()
			return nil, ErrPendingTransaction
		} else if !os.IsNotExist(err) {
			release()
			return nil, err
		}
	}
	return release, nil
}
