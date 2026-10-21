package core

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type idleCapture struct {
	mu sync.Mutex
	bytes.Buffer
}

func TestIdleKeepAliveInputBusyAndDisable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		writer := &idleCapture{}
		c := &Connection{writer: writer, done: make(chan struct{}), idleChanges: make(chan KeepAliveSettings, 1)}
		c.lastActivity.Store(time.Now().UnixNano())
		defer close(c.done)
		k := DefaultKeepAlive()
		k.IdleEnabled, k.IdleInterval, k.IdleText = true, 30, `ls \n`
		go c.idleKeepAlive(k)
		time.Sleep(29 * time.Second)
		c.Write(nil) // Empty terminal packets are not user activity.
		time.Sleep(time.Second)
		synctest.Wait()
		if writer.text() != "ls \n" {
			t.Fatal("empty write suppressed idle send")
		}
		c.SetManagedBusy(true)
		time.Sleep(90 * time.Second)
		synctest.Wait()
		if writer.text() != "ls \n" {
			t.Fatal("idle text leaked into transfer")
		}
		c.SetManagedBusy(false)
		time.Sleep(29 * time.Second)
		c.Write([]byte("typed"))
		time.Sleep(time.Second)
		synctest.Wait()
		if writer.text() != "ls \ntyped" {
			t.Fatal("idle send raced with input")
		}
		time.Sleep(29 * time.Second)
		synctest.Wait()
		if writer.text() != "ls \ntypedls \n" {
			t.Fatal("idle send never resumed")
		}
		k.IdleEnabled = false
		if err := c.UpdateIdleKeepAlive(k); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(time.Hour)
		synctest.Wait()
		if writer.text() != "ls \ntypedls \n" {
			t.Fatal("disabled idle timer still sent")
		}
	})
}

func TestSSHKeepAliveAndIdleSend(t *testing.T) {
	server := startSSH(t, "idle-test")
	// A negative global-request reply still proves the server is alive.
	server.rejectKeepAlive.Store(true)
	p := testProfile(server.address)
	p.KeepAlive.Interval, p.KeepAlive.MaxMissed = 1, 2
	p.KeepAlive.IdleEnabled, p.KeepAlive.IdleInterval, p.KeepAlive.IdleText = true, 1, `ls \n`
	c, err := Dial(context.Background(), p, Credentials{Password: "idle-test"}, testTrust(t), acceptHost)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	read := make(chan string, 16)
	go func() {
		for {
			b := make([]byte, 4)
			if _, err := io.ReadFull(c, b); err != nil {
				return
			}
			read <- string(b)
		}
	}()
	for i := 0; i < 2; i++ {
		select {
		case got := <-read:
			if got != "ls \n" {
				t.Fatalf("incorrect wire bytes: %q", got)
			}
		case <-time.After(4 * time.Second):
			t.Fatal("idle command never reached SSH server")
		}
	}
	if !c.Alive() || server.keepAliveRequests.Load() == 0 {
		t.Fatal("SSH heartbeat did not run alongside idle text")
	}
	k := p.KeepAlive
	k.IdleEnabled = false
	if err := c.UpdateIdleKeepAlive(k); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-read:
		t.Fatalf("disabled timer still sent %q", got)
	case <-time.After(1400 * time.Millisecond):
	}
	if !c.Alive() {
		t.Fatal("negative heartbeat response closed a live server")
	}
}

func TestSSHKeepAliveClosesUnresponsivePeer(t *testing.T) {
	server := startSSH(t, "idle-test")
	server.ignoreKeepAlive.Store(true)
	p := testProfile(server.address)
	p.KeepAlive.Interval, p.KeepAlive.MaxMissed = 1, 2
	c, err := Dial(context.Background(), p, Credentials{Password: "idle-test"}, testTrust(t), acceptHost)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	c.WaitClosed(ctx)
	if ctx.Err() != nil || c.Alive() || server.keepAliveRequests.Load() != 1 {
		t.Fatal("unresponsive SSH peer was not closed with one bounded pending probe")
	}
}

func (w *idleCapture) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Buffer.Write(p)
}
func (*idleCapture) Close() error   { return nil }
func (w *idleCapture) text() string { w.mu.Lock(); defer w.mu.Unlock(); return w.Buffer.String() }

func TestIdleKeepAliveLiveApplyRestartsInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		writer := &idleCapture{}
		c := &Connection{writer: writer, done: make(chan struct{}), idleChanges: make(chan KeepAliveSettings, 1)}
		c.lastActivity.Store(time.Now().UnixNano())
		defer close(c.done)
		k := DefaultKeepAlive()
		go c.idleKeepAlive(k)
		time.Sleep(10 * time.Second)
		k.IdleEnabled, k.IdleInterval, k.IdleText = true, 30, `ls \n`
		if err := c.UpdateIdleKeepAlive(k); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(29 * time.Second)
		synctest.Wait()
		if got := writer.text(); got != "" {
			t.Fatalf("sent before new idle interval: %q", got)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if got := writer.text(); got != "ls \n" {
			t.Fatalf("missing idle command: %q", got)
		}
	})
}
