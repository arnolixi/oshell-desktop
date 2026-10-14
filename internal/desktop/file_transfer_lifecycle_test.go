package desktop

import (
	"context"
	"fmt"
	"oshell-desktop/internal/core"
	"testing"
)

func TestOldTransferCallbacksCannotFinishOrOverwriteNewTask(t *testing.T) {
	a, p, _ := savedConnectionFixture(t)
	p.Kind = "sftp"
	p.InitialDirectory = "/"
	a.config.Profiles[0] = p
	if err := a.vault.PutFor(p, "saved-fixture-password", p.Identity()); err != nil {
		t.Fatal(err)
	}
	a.request(p, 0)
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 1 && a.tabs[0].active.files != nil && !a.tabs[0].active.files.busy && a.tabs[0].active.files.client != nil
	})
	pane := a.tabs[0].active
	type callbacks struct {
		task   *fileTransfer
		report func(core.TransferProgress)
	}
	captured := make(chan callbacks, 1)
	a.withTransfer(pane, func(ctx context.Context, client core.RemoteFileSystem) error {
		task := ctx.Value(fileTransferContextKey{}).(*fileTransfer)
		captured <- callbacks{task, a.transferProgress(pane, ctx)}
		_, err := client.Lstat("/")
		return err
	})
	waitOperator(t, a, func() bool { return !pane.files.busy })
	var old callbacks
	select {
	case old = <-captured:
	default:
		t.Fatal("first operation did not run")
	}
	started := make(chan struct{})
	a.withTransfer(pane, func(ctx context.Context, client core.RemoteFileSystem) error {
		a.transferProgress(pane, ctx)(core.TransferProgress{Bytes: 64, Files: 1})
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	waitOperator(t, a, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	})
	a.drain()
	current, before := pane.files.activeTransfer, pane.files.status
	old.report(core.TransferProgress{Bytes: 999999, Files: 999})
	a.transferDone(pane, old.task, fmt.Errorf("stale failure"))
	a.drain()
	if pane.files.activeTransfer != current || !pane.files.busy || pane.files.cancel == nil || pane.files.status != before {
		t.Fatal("old callback changed current operation")
	}
	pane.files.cancel()
	waitOperator(t, a, func() bool { return !pane.files.busy && pane.files.status == "传输已取消" })
	old.report(core.TransferProgress{Bytes: 888888})
	a.transferDone(pane, current, nil)
	a.drain()
	if pane.files.activeTransfer != nil || pane.files.status != "传输已取消" {
		t.Fatal("late completion replaced cancellation")
	}
}
