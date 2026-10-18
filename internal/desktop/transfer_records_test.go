package desktop

import (
	"bytes"
	"context"
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"testing"

	"oshell-desktop/internal/core"
)

func TestTransferRecordsRetentionAndReconnectState(t *testing.T) {
	log := &transferLog{}
	for i := 0; i < 600; i++ {
		log.records = append(log.records, &transferRecord{Status: "成功"})
	}
	for _, s := range []string{"已暂停", "等待", "传输中"} {
		log.records = append(log.records, &transferRecord{Status: s})
	}
	log.pruneLocked(false)
	if len(log.snapshot()) != 503 {
		t.Fatal(len(log.records))
	}
	old := &fileState{records: log, dir: "/directory", protocol: "SFTP"}
	old.limiter.Set(8192)
	next := reconnectFileState(old)
	if next.records != log || next.dir != old.dir || next.limiter.Rate() != 8192 {
		t.Fatal("reconnect lost local state")
	}
	log.pruneLocked(true)
	if len(log.records) != 3 {
		t.Fatal("cleared unfinished records")
	}
}

func TestTransferCheckpointClearedWhenResolvedHostChanges(t *testing.T) {
	f := &fileState{}
	first := core.SSHIdentity{Host: "first.example", User: "user", Port: 22}
	bindTransferIdentity(f, first)
	log := f.records
	log.records = []*transferRecord{{Status: "已暂停", Resume: true}}
	log.pending = "queued"
	bindTransferIdentity(f, first)
	if f.records != log {
		t.Fatal("same identity lost records")
	}
	second := first
	second.Host = "second.example"
	bindTransferIdentity(f, second)
	if f.records == log || len(f.records.snapshot()) != 0 || f.records.pending != "" {
		t.Fatal("checkpoint crossed actual SSH identity")
	}
}

func TestTransferPauseResumeRecordsAndReconnect(t *testing.T) {
	a, profile, _ := savedConnectionFixture(t)
	profile.Kind = "sftp"
	profile.InitialDirectory = "/"
	a.config.Profiles[0] = profile
	if err := a.vault.PutFor(profile, "saved-fixture-password", profile.Identity()); err != nil {
		t.Fatal(err)
	}
	a.request(profile, 0)
	waitOperator(t, a, func() bool {
		return len(a.tabs) == 1 && a.activePane().files != nil && !a.activePane().files.busy && a.activePane().files.client != nil
	})
	p := a.activePane()
	source := filepath.Join(t.TempDir(), "payload")
	data := bytes.Repeat([]byte("data"), 128<<10)
	os.WriteFile(source, data, 0600)
	p.files.limiter.Set(1 << 20)
	a.withTransfer(p, func(ctx context.Context, client core.RemoteFileSystem) error {
		task := ctx.Value(fileTransferContextKey{}).(*fileTransfer)
		return uploadUnique(ctx, client, []string{source, source}, "/", func(progress core.TransferProgress) {
			if progress.Bytes >= 64<<10 {
				task.pause.Store(true)
				p.files.cancel()
			}
		})
	})
	waitOperator(t, a, func() bool { return !p.files.busy })
	records := p.files.records.snapshot()
	if len(records) != 2 || records[0].Status != "已暂停" || records[1].Status != "未执行" {
		t.Fatalf("records: %+v; %s", records, p.files.status)
	}
	oldLog := p.files.records
	p.files.recordsOpen = true
	view := ui.NewTester(a.fileWindowView, 1080, 720)
	view.Frame()
	if _, ok := view.Find("继续"); !ok {
		t.Fatal("paused transfer has no resume button")
	}
	snapshot(t, view, "transfer-records.png")
	// A dead transport must reconnect using the saved identity and continue the
	// same checkpoint without creating another tab or losing completed records.
	p.conn.Close()
	a.resumeTransfer(p, records[0].ID)
	defer func() {
		if t.Failed() {
			next := a.activePane()
			t.Logf("reconnect: old=%v tabs=%d auth=%v status=%s records=%+v", next == p, len(a.tabs), a.authOpen, next.status, oldLog.snapshot())
			if next.files != nil {
				t.Logf("file busy=%v status=%s pending=%s", next.files.busy, next.files.status, oldLog.pending)
			}
		}
	}()
	waitOperator(t, a, func() bool {
		return a.activePane() != p && a.activePane().files != nil && !a.activePane().files.busy && len(oldLog.snapshot()) > 0 && oldLog.snapshot()[0].Status == "成功"
	})
	p = a.activePane()
	if len(a.tabs) != 1 || p.files.records != oldLog {
		t.Fatal("reconnect lost file-tab history")
	}
	target := filepath.Join(t.TempDir(), "download")
	a.withTransfer(p, func(ctx context.Context, c core.RemoteFileSystem) error {
		return c.Download(ctx, "/payload", target, false, nil)
	})
	waitOperator(t, a, func() bool { return !p.files.busy })
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("resumed content differs", err, p.files.status)
	}
}
