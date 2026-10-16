package desktop

import (
	"os"
	"oshell-desktop/internal/core"
	"sync"
	"testing"
	"time"
)

type stalledDirectoryClient struct {
	core.RemoteFileSystem
	started, closed, finish chan struct{}
	once                    sync.Once
}

func (c *stalledDirectoryClient) RealPath(string) (string, error) {
	close(c.started)
	<-c.finish
	return "/late", nil
}
func (c *stalledDirectoryClient) ReadDir(string) ([]os.FileInfo, error) { return nil, nil }
func (c *stalledDirectoryClient) Close() error                          { c.once.Do(func() { close(c.closed) }); return nil }

func TestDirectoryReadFollowsMovedPaneAfterSourceCloses(t *testing.T) {
	for _, queuedBeforeMove := range []bool{false, true} {
		t.Run(map[bool]string{false: "in-flight", true: "queued"}[queuedBeforeMove], func(t *testing.T) {
			store := &core.Store{Dir: t.TempDir()}
			source := newApp(store, core.DefaultConfig())
			destination := newApp(store, core.DefaultConfig())
			client := &stalledDirectoryClient{started: make(chan struct{}), closed: make(chan struct{}), finish: make(chan struct{})}
			p := &pane{ftp: &core.FTPSession{}, files: &fileState{client: client, lease: func() {}, dir: "/original"}}
			p.owner.Store(source)
			defer p.ftp.Close()
			defer client.Close()
			source.loadFiles(p, "/requested")
			select {
			case <-client.started:
			case <-time.After(time.Second):
				t.Fatal("read not started")
			}
			if queuedBeforeMove {
				close(client.finish)
				deadline := time.Now().Add(time.Second)
				for {
					source.eventMu.Lock()
					queued := len(source.events) != 0
					source.eventMu.Unlock()
					if queued {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("result not queued")
					}
					time.Sleep(time.Millisecond)
				}
			}
			p.owner.Store(destination)
			source.stopped.Store(true)
			source.drain() // OnClosed drains already queued callbacks.
			if !queuedBeforeMove {
				close(client.finish)
			}
			waitOperator(t, destination, func() bool { return !p.files.busy })
			if p.files.dir != "/late" || p.files.client != client || p.files.readCancel != nil {
				t.Fatal("result did not reach the new owner")
			}
			select {
			case <-client.closed:
				t.Fatal("live moved pane lost its file client")
			default:
			}
		})
	}
}

func TestDirectoryReadCancellationAndClosedPaneIgnoreLateResult(t *testing.T) {
	for _, dispose := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "dispose"}[dispose], func(t *testing.T) {
			a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
			client := &stalledDirectoryClient{started: make(chan struct{}), closed: make(chan struct{}), finish: make(chan struct{})}
			p := &pane{id: core.NewID(), ftp: &core.FTPSession{}, files: &fileState{client: client, lease: func() {}, dir: "/original"}}
			p.owner.Store(a)
			a.loadFiles(p, "/requested")
			select {
			case <-client.started:
			case <-time.After(time.Second):
				t.Fatal("read not started")
			}
			if dispose {
				a.dispose(p)
				p.files.status = "closed-state"
			} else {
				p.files.readCancel()
			}
			select {
			case <-client.closed:
			case <-time.After(time.Second):
				t.Fatal("cancel did not close reading client")
			}
			close(client.finish)
			until := time.Now().Add(time.Second)
			queued := false
			for time.Now().Before(until) {
				a.eventMu.Lock()
				queued = len(a.events) > 0
				a.eventMu.Unlock()
				if queued {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if !queued {
				t.Fatal("read did not settle")
			}
			a.drain()
			if p.files.dir != "/original" {
				t.Fatal("late directory result applied")
			}
			if dispose {
				p.disposal.Wait()
				if p.files.status != "closed-state" {
					t.Fatal("closed pane status overwritten")
				}
			} else {
				if p.files.busy || p.files.client != nil || p.files.readCancel != nil || p.files.status != "目录读取已取消" {
					t.Fatal("cancel did not reset read state")
				}
				p.ftp.Close()
			}
		})
	}
}
