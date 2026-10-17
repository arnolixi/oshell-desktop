package core

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestSwiftSharedProtocolRoundTrip(t *testing.T) {
	helper := os.Getenv("OSHELL_SWIFT_INTEROP_HELPER")
	if helper == "" {
		t.Skip("run scripts/test-sync-interop.py with the native OShell checkout")
	}
	dir := t.TempDir()
	swift := filepath.Join(dir, "swift.json")
	goFile := filepath.Join(dir, "go.json")
	back := filepath.Join(dir, "back.json")
	run := func(args ...string) {
		t.Helper()
		if output, err := exec.Command(helper, args...).CombinedOutput(); err != nil {
			t.Fatalf("Swift helper: %v %s", err, output)
		}
	}
	run("create", swift)
	data, err := os.ReadFile(swift)
	if err != nil {
		t.Fatal(err)
	}
	master := []byte("interop-fixture-master-🔐")
	doc, err := DecodeSharedDocument(data, master)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := doc.Configuration()
	if err != nil {
		t.Fatal(err)
	}
	v, err := OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = v.SetMaster(string(master)); err != nil {
		t.Fatal(err)
	}
	for _, p := range doc.Profiles {
		password, err := DecryptMacPassword(*p.Password, master, p.Definition)
		if err != nil || password != "fixture-"+p.Definition.Kind {
			t.Fatal("credential", err)
		}
		if err = v.PutFor(p.Definition, password, p.Password.Identity); err != nil {
			t.Fatal(err)
		}
	}
	for _, proxy := range doc.Proxies {
		p := ProxyCredentialProfile(proxy.Definition.Settings)
		password, err := DecryptMacPassword(*proxy.Password, master, p)
		if err != nil {
			t.Fatal(err)
		}
		if err = v.PutFor(p, password, proxy.Password.Identity); err != nil {
			t.Fatal(err)
		}
	}
	exported, _, err := v.ExportSharedDocument(context.Background(), cfg, master, &doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err = exported.Encode(master)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(goFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	run("roundtrip", goFile, back)
	stableData, err := os.ReadFile(back)
	if err != nil {
		t.Fatal(err)
	}
	stable, err := DecodeSharedDocument(stableData, master)
	if err != nil {
		t.Fatal(err)
	}
	if !sameShared(exported, stable) {
		a, _ := sharedFields(exported)
		b, _ := sharedFields(stable)
		for key, value := range a {
			if !equalSharedJSON(value, b[key]) {
				t.Errorf("round trip changes section %s", key)
			}
		}
	}
	run("edit", goFile, back)
	data, err = os.ReadFile(back)
	if err != nil {
		t.Fatal(err)
	}
	round, err := DecodeSharedDocument(data, master)
	if err != nil {
		t.Fatal(err)
	}
	if round.Profiles[0].Definition.Name != "Swift 修改" {
		t.Fatal("Swift edit lost")
	}
}

func TestSwiftMultiNodeSyncTransports(t *testing.T) {
	helper := os.Getenv("OSHELL_SWIFT_INTEROP_HELPER")
	if helper == "" {
		t.Skip("Swift interoperability helper required")
	}
	for _, backend := range []string{"directory", "webdav"} {
		t.Run(backend, func(t *testing.T) {
			ctx := context.Background()
			master := []byte("interop-fixture-master-🔐")
			run := func(args ...string) {
				t.Helper()
				if output, err := exec.Command(helper, args...).CombinedOutput(); err != nil {
					t.Fatalf("Swift: %v %s", err, output)
				}
			}
			var transport SyncTransport
			var address string
			if backend == "directory" {
				address = t.TempDir()
				transport = DirectoryTransport{address}
			} else {
				var mu sync.Mutex
				var data []byte
				tag := ""
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()
					user, password, ok := r.BasicAuth()
					if !ok || user != "test-user" || password != "test-password" {
						w.WriteHeader(401)
						return
					}
					if r.URL.Path != "/vault/oshell-vault.json" {
						http.NotFound(w, r)
						return
					}
					switch r.Method {
					case "GET":
						if data == nil {
							w.WriteHeader(404)
							return
						}
						w.Header().Set("ETag", tag)
						w.Write(data)
					case "PUT":
						if data == nil && r.Header.Get("If-None-Match") != "*" || data != nil && r.Header.Get("If-Match") != tag {
							w.WriteHeader(412)
							return
						}
						payload, err := io.ReadAll(io.LimitReader(r.Body, MaxSharedVault+1))
						if err != nil || len(payload) > MaxSharedVault {
							w.WriteHeader(400)
							return
						}
						data = payload
						sum := sha256.Sum256(data)
						tag = fmt.Sprintf("\"%x\"", sum)
						w.WriteHeader(204)
					default:
						w.WriteHeader(405)
					}
				}))
				defer server.Close()
				address = server.URL + "/vault"
				var err error
				transport, err = NewWebDAVTransport(address, "test-user", "test-password", true)
				if err != nil {
					t.Fatal(err)
				}
			}
			fixture := filepath.Join(t.TempDir(), "swift.json")
			run("create", fixture)
			data, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}
			if err = transport.Put(ctx, data, ""); err != nil {
				t.Fatal(err)
			}
			node := func() (*SharedSyncEngine, Config) {
				dir := t.TempDir()
				store := &Store{Dir: dir}
				cfg := DefaultConfig()
				if err := store.Save(cfg); err != nil {
					t.Fatal(err)
				}
				vault, err := OpenVault(dir)
				if err != nil {
					t.Fatal(err)
				}
				if err = vault.SetMaster(string(master)); err != nil {
					t.Fatal(err)
				}
				engine := &SharedSyncEngine{Store: store, Vault: vault, Transport: transport, Replica: SharedReplicaStore{Directory: dir, Target: address}}
				plan, err := engine.Prepare(ctx, cfg, master, false)
				if err != nil {
					t.Fatal(err)
				}
				if plan.InitialChoiceRequired {
					if err = plan.ChooseInitial(false); err != nil {
						t.Fatal(err)
					}
				}
				cfg, err = plan.Commit(ctx, nil, master)
				if err != nil {
					t.Fatal(err)
				}
				return engine, cfg
			}
			a, local := node()
			b, other := node()
			// Swift and Go edit different sessions from a common baseline.
			local.Profiles[1].Name = "Go 修改"
			if err = a.Store.Save(local); err != nil {
				t.Fatal(err)
			}
			run(backend+"-edit", address)
			plan, err := a.Prepare(ctx, local, master, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Conflicts) != 0 {
				t.Fatalf("independent edits conflict: %+v", plan.Conflicts)
			}
			local, err = plan.Commit(ctx, nil, master)
			if err != nil {
				t.Fatal(err)
			}
			if local.Profiles[0].Name != "Swift 修改" || local.Profiles[1].Name != "Go 修改" {
				t.Fatal("independent edit lost")
			}
			// A stale node editing the same session must not overwrite Swift's change.
			other.Profiles[0].Name = "stale node edit"
			if err = b.Store.Save(other); err != nil {
				t.Fatal(err)
			}
			conflict, err := b.Prepare(ctx, other, master, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(conflict.Conflicts) != 1 {
				t.Fatalf("wanted conflict, got %+v", conflict.Conflicts)
			}
			before, err := transport.Fetch(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = conflict.Commit(ctx, nil, master); err == nil {
				t.Fatal("unresolved overwrite accepted")
			}
			after, err := transport.Fetch(ctx)
			if err != nil || after.ETag != before.ETag {
				t.Fatal("failed conflict changed shared object", err)
			}
			// No-change sync must not write again just because Swift normalized JSON.
			plan, err = a.Prepare(ctx, local, master, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = plan.Commit(ctx, nil, master); err != nil {
				t.Fatal(err)
			}
			stable, err := transport.Fetch(ctx)
			if err != nil || stable.ETag != after.ETag {
				t.Fatal("unchanged sync rewrote remote", err)
			}
			verify := filepath.Join(t.TempDir(), "final.json")
			if err = os.WriteFile(verify, stable.Data, 0600); err != nil {
				t.Fatal(err)
			}
			run("verify", verify)
			for _, p := range local.Profiles {
				secret, err := a.Vault.GetFor(p, p.Identity())
				if err != nil || secret != "fixture-"+p.Kind {
					t.Fatal("credential lost", err)
				}
			}
		})
	}
}

func TestSwiftReadsNewDesktopDocumentWithoutSpuriousChanges(t *testing.T) {
	helper := os.Getenv("OSHELL_SWIFT_INTEROP_HELPER")
	if helper == "" {
		t.Skip("Swift helper required")
	}
	cfg := DefaultConfig()
	p := NewProfile()
	p.Host = "fixture.invalid"
	p.Username = "fixture"
	cfg.Profiles = []Profile{p}
	v, err := OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	master := []byte("interop-fixture-master-🔐")
	if err = v.SetMaster(string(master)); err != nil {
		t.Fatal(err)
	}
	if err = v.PutFor(p, "fixture-secret", p.Identity()); err != nil {
		t.Fatal(err)
	}
	doc, _, err := v.ExportSharedDocument(context.Background(), cfg, master, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := doc.Encode(master)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "input.json")
	output := filepath.Join(t.TempDir(), "output.json")
	if err = os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	if text, err := exec.Command(helper, "native-roundtrip", input, output).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, text)
	}
	data, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeSharedDocument(data, master)
	if err != nil {
		t.Fatal(err)
	}
	if !sameShared(doc, back) {
		a, _ := sharedFields(doc)
		b, _ := sharedFields(back)
		for key, value := range a {
			if !equalSharedJSON(value, b[key]) {
				t.Errorf("native roundtrip changes section %s", key)
				if key == "preferences" {
					for k, v := range doc.Preferences {
						if !equalSharedJSON(v, back.Preferences[k]) {
							t.Logf("preference %s: %s -> %s", k, v, back.Preferences[k])
						}
					}
				}
			}
		}
	}
	secret, err := DecryptMacPassword(*back.Profiles[0].Password, master, back.Profiles[0].Definition)
	if err != nil || secret != "fixture-secret" {
		t.Fatal("Go-origin credential lost", err)
	}
}
