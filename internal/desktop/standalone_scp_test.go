package desktop

import (
	"context"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
	"path/filepath"
	"testing"
)

func TestStandaloneSCPFailureRecordedWithoutSFTP(t *testing.T) {
	log := &transferLog{}
	task := &fileTransfer{}
	client := &recordedFileSystem{RemoteFileSystem: &standaloneSCPFileSystem{connection: &core.Connection{}}, log: log, task: task, protocol: "SCP"}
	if err := client.Upload(context.Background(), "fixture", "/target", nil); err == nil {
		t.Fatal("closed connection accepted")
	}
	if err := client.Download(context.Background(), "/source", filepath.Join(t.TempDir(), "target"), false, nil); err == nil {
		t.Fatal("closed connection accepted")
	}
	client.finish()
	records := log.snapshot()
	if len(records) != 2 {
		t.Fatal(records)
	}
	for _, r := range records {
		if r.Status != "失败" || r.Protocol != "SCP" || r.Error == "" || r.Resume {
			t.Fatalf("incorrect SCP result: %+v", r)
		}
	}
}

func TestStandaloneSCPRemainsAvailableWithoutBrowsingClient(t *testing.T) {
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
	f := pane.files
	f.client.Close()
	f.lease()
	f.client = nil
	f.lease = nil
	a.openStandaloneSCP(pane)
	if a.features.scpDraft == nil || !a.modalActive() {
		t.Fatal("SCP requires a browsing client")
	}
	tt := ui.NewTester(a.fileWindowView, 1080, 720)
	if !tt.HasText("SCP 文件传输") {
		t.Fatal("missing SCP dialog")
	}
	if err := tt.Click("上传…"); err != nil {
		t.Fatal(err)
	}
	if a.features.scpDraft == nil || a.features.scpDraft.err == "" {
		t.Fatal("invalid remote path accepted")
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	ran := make(chan struct{})
	a.withFileOperation(pane, false, func(ctx context.Context) error { close(ran); return nil })
	waitOperator(t, a, func() bool { return !f.busy })
	select {
	case <-ran:
	default:
		t.Fatal("direct operation not executed")
	}
	if f.client != nil || f.status != "操作完成" {
		t.Fatal("direct operation tried to reload unavailable SFTP")
	}
}
