package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestStateLockSubprocess(t *testing.T) {
	if dir := os.Getenv("OSHELL_STATE_LOCK_CHILD"); dir != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		release, err := lockStateDirectory(ctx, dir, false)
		if err == nil {
			release()
			t.Fatal("child acquired parent's lock")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		return
	}
	dir := t.TempDir()
	release, err := lockStateDirectory(context.Background(), dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	cmd := exec.Command(os.Args[0], "-test.run=^TestStateLockSubprocess$")
	cmd.Env = append(os.Environ(), "OSHELL_STATE_LOCK_CHILD="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestConcurrentStoreWritersRetainWinner(t *testing.T) {
	dir := t.TempDir()
	initial := &Store{Dir: dir}
	if err := initial.Save(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	const writers = 12
	stores := make([]*Store, writers)
	configs := make([]Config, writers)
	for i := range stores {
		stores[i] = &Store{Dir: dir}
		var err error
		configs[i], err = stores[i].Load()
		if err != nil {
			t.Fatal(err)
		}
		p := NewProfile()
		p.Name = p.ID
		p.Host = "concurrent.invalid"
		configs[i].Profiles = append(configs[i].Profiles, p)
	}
	start := make(chan struct{})
	type result struct {
		index int
		err   error
	}
	results := make(chan result, writers)
	for i := range stores {
		go func(i int) { <-start; results <- result{i, stores[i].Save(configs[i])} }(i)
	}
	close(start)
	winner := -1
	for range writers {
		r := <-results
		if r.err == nil {
			if winner != -1 {
				t.Fatal("two stale writers succeeded")
			}
			winner = r.index
		} else if !errors.Is(r.err, ErrConfigurationChanged) {
			t.Fatal(r.err)
		}
	}
	if winner == -1 {
		t.Fatal("no writer succeeded")
	}
	got, err := (&Store{Dir: dir}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles[len(got.Profiles)-1].ID != configs[winner].Profiles[len(configs[winner].Profiles)-1].ID {
		t.Fatal("winner lost")
	}
}

func TestForeignPendingTransactionBlocksWriters(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	cfg := DefaultConfig()
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	v, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, transactionFile), []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.Save(cfg); !errors.Is(err, ErrPendingTransaction) {
		t.Fatal(err)
	}
	p := NewProfile()
	p.Host = "pending.invalid"
	if err = v.Put(p, "test-only"); !errors.Is(err, ErrPendingTransaction) {
		t.Fatal(err)
	}
}
