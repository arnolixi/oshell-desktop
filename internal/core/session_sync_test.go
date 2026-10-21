package core

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestSessionSyncKeepsClientSettingsAndRemoteCompatibility(t *testing.T) {
	ctx := context.Background()
	engine, cfg, master := syncEngineFixture(t, t.TempDir())
	cfg.Settings.FontSize = 19
	cfg.Settings.LinksVisible = false
	cfg.Settings.InterfaceTheme = "dark"
	cfg.SessionDefaults.SSHPort = 2219
	cfg.QuickCommands = []QuickCommand{{ID: NewID(), Name: "local", Text: "echo local"}}
	if err := engine.Store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := engine.Vault.Put(cfg.Profiles[0], "session-secret"); err != nil {
		t.Fatal(err)
	}
	legacy, _, err := engine.Vault.ExportSharedDocument(ctx, cfg, master, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	legacy.Preferences["fontSize"] = json.RawMessage(`23`)
	legacy.Links.Visible = true
	legacy.Defaults.SSHPort = 2223
	legacy.QuickCommands = []QuickCommand{{ID: NewID(), Name: "remote", Text: "echo remote"}}
	legacy.Profiles[0].Definition.Name = "remote session"
	encrypted, err := legacy.Encode(master)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Transport.Put(ctx, encrypted, ""); err != nil {
		t.Fatal(err)
	}
	plan, err := engine.Prepare(ctx, cfg, master, false)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.InitialChoiceRequired {
		t.Fatal("missing initial confirmation")
	}
	if err = plan.ChooseInitial(false); err != nil {
		t.Fatal(err)
	}
	next, err := plan.Commit(ctx, nil, master)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(next.Settings, cfg.Settings) || !reflect.DeepEqual(next.QuickCommands, cfg.QuickCommands) ||
		!reflect.DeepEqual(next.HighlightSets, cfg.HighlightSets) || !reflect.DeepEqual(next.SessionDefaults, cfg.SessionDefaults) {
		t.Fatal("shared session download overwrote local client configuration")
	}
	if next.Profiles[0].Name != "remote session" {
		t.Fatal("remote session not applied")
	}
	if secret, err := engine.Vault.GetFor(next.Profiles[0], next.Profiles[0].Identity()); err != nil || secret != "session-secret" {
		t.Fatal("session password lost", err)
	}
	before, _ := engine.Transport.Fetch(ctx)
	fingerprint := SyncLocalFingerprint(engine.Store, engine.Vault)
	next.Settings.FontSize++
	next.Settings.LinksVisible = !next.Settings.LinksVisible
	next.QuickCommands[0].Text = "echo changed locally"
	next.SessionDefaults.SSHPort++
	if err = engine.Store.Save(next); err != nil {
		t.Fatal(err)
	}
	if fingerprint == "" || SyncLocalFingerprint(engine.Store, engine.Vault) != fingerprint {
		t.Fatal("client-only edit marked session data dirty")
	}
	plan, err = engine.Prepare(ctx, next, master, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.InitialChoiceRequired || len(plan.Conflicts) != 0 {
		t.Fatal("client-only edit triggered merge")
	}
	next, err = plan.Commit(ctx, nil, master)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := engine.Transport.Fetch(ctx)
	if before.ETag != after.ETag || !bytes.Equal(before.Data, after.Data) {
		t.Fatal("client-only edit rewrote remote object")
	}
	// An old client changes its settings; our session edit must retain that
	// latest fetched compatibility payload instead of restoring an old baseline.
	remote, err := DecodeSharedDocument(after.Data, master)
	if err != nil {
		t.Fatal(err)
	}
	remote.Preferences["fontSize"] = json.RawMessage(`25`)
	raw, err := remote.Encode(master)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Transport.Put(ctx, raw, after.ETag); err != nil {
		t.Fatal(err)
	}
	next.Profiles[0].Name = "desktop session edit"
	if err = engine.Store.Save(next); err != nil {
		t.Fatal(err)
	}
	if SyncLocalFingerprint(engine.Store, engine.Vault) == fingerprint {
		t.Fatal("session edit did not change fingerprint")
	}
	plan, err = engine.Prepare(ctx, next, master, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 0 {
		t.Fatal("legacy setting edit caused conflict")
	}
	next, err = plan.Commit(ctx, nil, master)
	if err != nil {
		t.Fatal(err)
	}
	after, _ = engine.Transport.Fetch(ctx)
	published, err := DecodeSharedDocument(after.Data, master)
	if err != nil {
		t.Fatal(err)
	}
	if string(published.Preferences["fontSize"]) != "25" || published.Defaults.SSHPort != 2223 ||
		!published.Links.Visible || !reflect.DeepEqual(published.QuickCommands, legacy.QuickCommands) {
		t.Fatal("remote compatibility sections were overwritten")
	}
	if next.Settings.FontSize != 20 || next.SessionDefaults.SSHPort != 2220 {
		t.Fatal("legacy remote settings entered local configuration")
	}
}

func TestSessionSyncInitialMergePreservesDependenciesAndPasswords(t *testing.T) {
	ctx := context.Background()
	target := t.TempDir()
	a, left, master := syncEngineFixture(t, target)
	b, right, _ := syncEngineFixture(t, target)
	setup := func(e *SharedSyncEngine, cfg *Config, name string) {
		t.Helper()
		proxy := NewProxyProfile()
		proxy.Settings.Host = name + ".invalid"
		cfg.Proxies = []ProxyProfile{proxy}
		cfg.Profiles[0].ProxyID = proxy.ID()
		cfg.Profiles[0].Name = name
		cfg.Profiles[0].Group = name
		cfg.Directories = []string{name, "Links/" + name}
		link := SessionLink{ID: NewID(), ProfileID: cfg.Profiles[0].ID, Name: name, Folder: name}
		cfg.Links = []SessionLink{link}
		if err := e.Store.Save(*cfg); err != nil {
			t.Fatal(err)
		}
		if err := e.Vault.Put(cfg.Profiles[0], name+"-secret"); err != nil {
			t.Fatal(err)
		}
		if err := e.Vault.Put(ProxyCredentialProfile(proxy.Settings), name+"-proxy-secret"); err != nil {
			t.Fatal(err)
		}
	}
	setup(a, &left, "left")
	setup(b, &right, "right")
	plan, err := a.Prepare(ctx, left, master, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Commit(ctx, nil, master); err != nil {
		t.Fatal(err)
	}
	plan, err = b.Prepare(ctx, right, master, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = plan.ChooseInitial(true); err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 0 {
		t.Fatal("disjoint session dependencies caused conflicts", plan.Conflicts)
	}
	merged, err := plan.Commit(ctx, nil, master)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Profiles) != 2 || len(merged.Proxies) != 2 || len(merged.Links) != 2 {
		t.Fatal("initial merge lost dependencies")
	}
	for _, p := range merged.Profiles {
		if secret, err := b.Vault.GetFor(p, p.Identity()); err != nil || secret != p.Name+"-secret" {
			t.Fatal("merged session password lost", err)
		}
	}
	for _, p := range merged.Proxies {
		profile := ProxyCredentialProfile(p.Settings)
		if secret, err := b.Vault.GetFor(profile, profile.Identity()); err != nil || secret == "" {
			t.Fatal("merged proxy password lost", err)
		}
	}
}

func TestSessionSyncNewTargetDoesNotExportClientSettings(t *testing.T) {
	e, cfg, master := syncEngineFixture(t, t.TempDir())
	cfg.Settings.FontSize = 29
	cfg.SessionDefaults.SSHPort = 2229
	cfg.Settings.UpdateRepository = "example/client-local"
	if err := e.Store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	plan, err := e.Prepare(context.Background(), cfg, master, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Commit(context.Background(), nil, master); err != nil {
		t.Fatal(err)
	}
	object, _ := e.Transport.Fetch(context.Background())
	doc, err := DecodeSharedDocument(object.Data, master)
	if err != nil {
		t.Fatal(err)
	}
	if string(doc.Preferences["fontSize"]) == "29" || doc.Defaults.SSHPort == 2229 ||
		string(doc.Preferences["updateRepository"]) == `"example/client-local"` {
		t.Fatal("new target exported local client settings")
	}
	before := SyncLocalFingerprint(e.Store, e.Vault)
	if err := e.Vault.Put(SyncOptions{Backend: "webdav", Address: "https://example.invalid", Username: "fixture"}.CredentialProfile(), "webdav-secret"); err != nil {
		t.Fatal(err)
	}
	if SyncLocalFingerprint(e.Store, e.Vault) != before {
		t.Fatal("WebDAV login marked sessions dirty")
	}
	if err := e.Vault.Put(cfg.Profiles[0], "changed-session-secret"); err != nil {
		t.Fatal(err)
	}
	if SyncLocalFingerprint(e.Store, e.Vault) == before {
		t.Fatal("session password change was missed")
	}
}
