package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestSharedWebDAVMultiNodeMergeAndStaleWrite(t *testing.T) {
	var mu sync.Mutex
	var data []byte
	tag, writes := "", 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		user, password, ok := r.BasicAuth()
		if !ok || user != "fixture-user" || password != "fixture-password" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/vault/oshell-vault.json" {
			w.WriteHeader(404)
			return
		}
		if r.Method == "PROPFIND" && r.URL.Path == "/vault/oshell/" {
			writeDAVCollection(w, r.URL.Path)
			return
		}
		if r.URL.Path != "/vault/oshell/oshell-vault.json" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case "GET":
			if data == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("ETag", tag)
			w.Write(data)
		case "PUT":
			if data == nil && r.Header.Get("If-None-Match") != "*" || data != nil && r.Header.Get("If-Match") != tag {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, MaxSharedVault+1))
			if err != nil || len(body) > MaxSharedVault {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			data = body
			writes++
			tag = fmt.Sprintf(`"revision-%d"`, writes)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	ctx := context.Background()
	a, ac, master := syncEngineFixture(t, server.URL)
	b, bc, _ := syncEngineFixture(t, server.URL)
	ac.Settings.FontSize, bc.Settings.FontSize = 18, 22
	ac.SessionDefaults.SSHPort, bc.SessionDefaults.SSHPort = 2218, 2222
	for _, engine := range []*SharedSyncEngine{a, b} {
		transport, err := NewWebDAVTransport(server.URL+"/vault", "fixture-user", "fixture-password", true)
		if err != nil {
			t.Fatal(err)
		}
		engine.Transport = transport
	}
	second := NewProfile()
	second.Host = "second.fixture.invalid"
	ac.Profiles = append(ac.Profiles, second)
	save := func(engine *SharedSyncEngine, cfg Config) {
		t.Helper()
		if err := engine.Store.Save(cfg); err != nil {
			t.Fatal(err)
		}
	}
	prepare := func(engine *SharedSyncEngine, cfg Config) *SharedSyncPlan {
		t.Helper()
		plan, err := engine.Prepare(ctx, cfg, master, true)
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	commit := func(plan *SharedSyncPlan) Config {
		t.Helper()
		cfg, err := plan.Commit(ctx, nil, master)
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	save(a, ac)
	save(b, bc)
	ac = commit(prepare(a, ac))
	plan := prepare(b, bc)
	if !plan.InitialChoiceRequired {
		t.Fatal("missing first-sync confirmation")
	}
	if err := plan.ChooseInitial(false); err != nil {
		t.Fatal(err)
	}
	bc = commit(plan)
	if bc.Settings.FontSize != 22 || bc.SessionDefaults.SSHPort != 2222 {
		t.Fatal("WebDAV download changed local client settings")
	}
	ac.Profiles[0].Name, bc.Profiles[1].Name = "A edit", "B edit"
	save(a, ac)
	save(b, bc)
	ac = commit(prepare(a, ac))
	bc = commit(prepare(b, bc))
	if bc.Profiles[0].Name != "A edit" || bc.Profiles[1].Name != "B edit" {
		t.Fatal("independent session edits did not merge")
	}
	before, err := b.Transport.Fetch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bc.Settings.FontSize = 24
	bc.Settings.LinksVisible = !bc.Settings.LinksVisible
	save(b, bc)
	bc = commit(prepare(b, bc))
	after, err := b.Transport.Fetch(ctx)
	if err != nil || before.ETag != after.ETag {
		t.Fatal("client-only WebDAV edit uploaded again", err)
	}
	ac = commit(prepare(a, ac))
	bc.Profiles[1].Name = "B pending"
	save(b, bc)
	stale := prepare(b, bc)
	ac.Profiles[0].Name = "A newer"
	save(a, ac)
	ac = commit(prepare(a, ac))
	if _, err := stale.Commit(ctx, nil, master); err == nil {
		t.Fatal("stale WebDAV upload accepted")
	}
	bc = commit(prepare(b, bc))
	if bc.Profiles[0].Name != "A newer" || bc.Profiles[1].Name != "B pending" {
		t.Fatal("retry lost a node's edit")
	}
}
