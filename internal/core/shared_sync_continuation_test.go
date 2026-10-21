package core

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

type continuationTransport struct {
	SyncTransport
	afterPut   func()
	afterFetch func()
	putError   error
}

func (s continuationTransport) Put(ctx context.Context, data []byte, etag string) error {
	if s.putError != nil {
		return s.putError
	}
	if err := s.SyncTransport.Put(ctx, data, etag); err != nil {
		return err
	}
	if s.afterPut != nil {
		s.afterPut()
	}
	return nil
}

func (s continuationTransport) Fetch(ctx context.Context) (*SharedObject, error) {
	object, err := s.SyncTransport.Fetch(ctx)
	if err == nil && s.afterFetch != nil {
		s.afterFetch()
	}
	return object, err
}

func TestSharedSyncUploadContinuation(t *testing.T) {
	for _, mode := range []string{"session-edit", "password-edit", "cancel", "first-merge"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			e, cfg, master := syncEngineFixture(t, t.TempDir())
			plan, err := e.Prepare(ctx, cfg, master, true)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err = plan.Commit(ctx, nil, master)
			if err != nil {
				t.Fatal(err)
			}
			// The remote adds a session that has not yet been applied locally.
			object, err := e.Transport.Fetch(ctx)
			if err != nil {
				t.Fatal(err)
			}
			remote, err := DecodeSharedDocument(object.Data, master)
			if err != nil {
				t.Fatal(err)
			}
			added := remote.Profiles[0]
			added.Definition.ID, added.Definition.Name = NewID(), "remote-only"
			remote.Profiles = append(remote.Profiles, added)
			data, err := remote.Encode(master)
			if err != nil {
				t.Fatal(err)
			}
			if err = e.Transport.Put(ctx, data, object.ETag); err != nil {
				t.Fatal(err)
			}
			copied := cfg.Profiles[0]
			copied.ID, copied.Name = NewID(), "local-copy"
			cfg.Profiles = append(cfg.Profiles, copied)
			if err = e.Store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err = e.Vault.Put(copied, "before-upload"); err != nil {
				t.Fatal(err)
			}
			if mode == "first-merge" {
				if err = os.Remove(e.Replica.path()); err != nil {
					t.Fatal(err)
				}
			}
			plan, err = e.Prepare(ctx, cfg, master, false)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "first-merge" {
				if err = plan.ChooseInitial(true); err != nil {
					t.Fatal(err)
				}
			}
			transport := e.Transport
			commitContext, cancel := context.WithCancel(ctx)
			defer cancel()
			e.Transport = continuationTransport{SyncTransport: transport, afterPut: func() {
				if mode == "cancel" {
					cancel()
					return
				}
				if mode == "password-edit" {
					if err := e.Vault.Put(copied, "after-upload"); err != nil {
						t.Fatal(err)
					}
					return
				}
				cfg.Profiles[1].Host = "edited-after-upload.invalid"
				changes, err := e.Vault.PrepareCredentialEdit(&copied, cfg.Profiles[1], "", true, cfg.Profiles[1].Identity())
				if err != nil {
					t.Fatal(err)
				}
				revision, err := e.Store.Revision()
				if err != nil {
					t.Fatal(err)
				}
				if err = e.Store.CommitCredentialEdit(ctx, cfg, e.Vault, changes, revision, e.Vault.Revision()); err != nil {
					t.Fatal(err)
				}
			}}
			if _, err = plan.Commit(commitContext, nil, master); !errors.Is(err, ErrSyncDeferred) {
				t.Fatal("successful upload did not preserve continuation", err)
			}
			diagnostic, err := e.Diagnostics().Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			last := diagnostic.Entries[len(diagnostic.Entries)-1]
			if mode != "cancel" && (last.Event != SyncQueued || last.Failure != nil) {
				t.Fatal("continuation misreported as damaged data")
			}
			replica, err := e.Replica.Load(master)
			if err != nil {
				t.Fatal(err)
			}
			if !sameShared(*replica.Baseline, plan.originalLocal) {
				t.Fatal("continuation must contain the pre-merge local snapshot")
			}
			if e.ConflictStore().Exists() {
				t.Fatal("published choices left a stale conflict draft")
			}
			e.Transport = transport
			cfg, err = e.Store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Profiles) != 2 {
				t.Fatal("interrupted commit changed local configuration")
			}
			plan, err = e.Prepare(ctx, cfg, master, false)
			if err != nil {
				t.Fatal(err)
			}
			if plan.InitialChoiceRequired || len(plan.Conflicts) != 0 {
				t.Fatal("false conflict after successful upload")
			}
			cfg, err = plan.Commit(ctx, nil, master)
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Profiles) != 3 {
				t.Fatal("unapplied remote addition was lost")
			}
			for _, p := range cfg.Profiles {
				if p.ID != copied.ID {
					continue
				}
				if (mode == "session-edit" || mode == "first-merge") && p.Host != "edited-after-upload.invalid" {
					t.Fatal("new local edit lost")
				}
				want := "before-upload"
				if mode == "password-edit" {
					want = "after-upload"
				}
				if secret, err := e.Vault.GetFor(p, p.Identity()); err != nil || secret != want {
					t.Fatal("credential binding lost", err)
				}
			}
			// Completing the retry must restore the ordinary no-op behavior.
			before, err := transport.Fetch(ctx)
			if err != nil {
				t.Fatal(err)
			}
			plan, err = e.Prepare(ctx, cfg, master, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = plan.Commit(ctx, nil, master); err != nil {
				t.Fatal(err)
			}
			after, err := transport.Fetch(ctx)
			if err != nil || before.ETag != after.ETag {
				t.Fatal("unchanged retry rewrote remote", err)
			}
		})
	}
}

func TestSharedSyncContinuationSafety(t *testing.T) {
	for _, mode := range []string{"failed-put", "download-only", "locked", "changed-master"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			e, cfg, master := syncEngineFixture(t, t.TempDir())
			plan, err := e.Prepare(ctx, cfg, master, true)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err = plan.Commit(ctx, nil, master)
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(e.Replica.path())
			if err != nil {
				t.Fatal(err)
			}
			if mode == "download-only" {
				object, err := e.Transport.Fetch(ctx)
				if err != nil {
					t.Fatal(err)
				}
				d, err := DecodeSharedDocument(object.Data, master)
				if err != nil {
					t.Fatal(err)
				}
				d.Profiles[0].Definition.Name = "remote-change"
				data, err := d.Encode(master)
				if err != nil {
					t.Fatal(err)
				}
				if err = e.Transport.Put(ctx, data, object.ETag); err != nil {
					t.Fatal(err)
				}
			} else {
				cfg.Profiles[0].Name = "local-change"
				if err = e.Store.Save(cfg); err != nil {
					t.Fatal(err)
				}
			}
			plan, err = e.Prepare(ctx, cfg, master, false)
			if err != nil {
				t.Fatal(err)
			}
			wrapper := continuationTransport{SyncTransport: e.Transport}
			switch mode {
			case "failed-put":
				wrapper.putError = errors.New("fixture failed PUT")
			case "download-only":
				wrapper.afterFetch = func() {
					cfg.Profiles[0].Name = "new-local-change"
					if err := e.Store.Save(cfg); err != nil {
						t.Fatal(err)
					}
				}
			case "locked":
				wrapper.afterPut = func() { e.Vault.Lock() }
			case "changed-master":
				wrapper.afterPut = func() {
					if err := e.Vault.SetMaster("different-fixture-master"); err != nil {
						t.Fatal(err)
					}
				}
			}
			e.Transport = wrapper
			if _, err = plan.Commit(ctx, nil, master); err == nil || errors.Is(err, ErrSyncDeferred) {
				t.Fatal("unsafe continuation accepted", err)
			}
			after, err := os.ReadFile(e.Replica.path())
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("unsafe continuation rewrote baseline", err)
			}
		})
	}
}
