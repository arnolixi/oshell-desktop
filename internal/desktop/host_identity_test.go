package desktop

import (
	"oshell-desktop/internal/core"
	"strings"
	"testing"
)

func TestIntegratedHostnameAndPassiveNestedHost(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p, _ := operatorPane(t, a, "profile-alias")
	p.profile.TitleMode = "shellIntegration"
	a.initializeHostname(p)
	a.receiveHostIdentity(p, core.HostIdentity{Hostname: "outer-real", Address: "10.20.0.1"})
	a.receiveHostnameHint(p, "prompt-alias")
	if p.hostname != "outer-real" || p.hostObservation.Identity.Address != "10.20.0.1" {
		t.Fatal("prompt replaced integration")
	}
	a.receiveHostnameHint(p, "inner-host")
	if p.hostname != "inner-host" || p.hostObservation.Identity.Address != "" {
		t.Fatal("nested host retained old IP")
	}
	p.profile.TitleMode = "passive"
	p.hostObservation = core.HostObservation{}
	a.initializeHostname(p)
	a.receiveHostIdentity(p, core.HostIdentity{Hostname: "ignored", Address: "10.1.1.1"})
	if p.hostname != "主机待识别" {
		t.Fatal("passive accepted metadata")
	}
	a.receiveHostnameHint(p, "observed")
	if p.hostname != "observed" || p.profile.Host != "profile-alias" {
		t.Fatal("display observation changed connection identity")
	}
}

func TestActiveProbeRequiresIdleOptInAndStopsAfterIntegration(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p, c := operatorPane(t, a, "fixture")
	p.term.Feed([]byte("user@host:~$ "))
	p.profile.TitleMode = "shellIntegration"
	a.initializeHostname(p)
	a.scheduleActiveHostProbe(p, "user@host:~$")
	if p.hostProbe.work != nil || c.text() != "" {
		t.Fatal("default mode injected probe")
	}
	p.hostObservation.Mode = "activeProbe"
	p.hostProbe.editing = true
	a.scheduleActiveHostProbe(p, "user@host:~$")
	if p.hostProbe.work != nil {
		t.Fatal("probe while editing")
	}
	p.hostProbe.editing = false
	a.scheduleActiveHostProbe(p, "user@host:~$")
	waitOperator(t, a, func() bool { return c.text() != "" })
	if p.hostProbe.token == "" || !strings.HasPrefix(c.text(), " sh -c ") {
		t.Fatal("missing probe")
	}
	token := p.hostProbe.token
	a.receiveHostTitle(p, token+"reported|192.0.2.67|")
	if p.hostname != "reported" || p.hostObservation.Identity.Address != "192.0.2.67" {
		t.Fatal("probe reply not applied")
	}
	a.receiveHostIdentity(p, core.HostIdentity{Hostname: "integrated", Address: "192.0.2.68"})
	if p.hostObservation.AllowsActiveProbe() || p.hostProbe.timeout != nil || p.hostProbe.work != nil {
		t.Fatal("integration did not stop future probes")
	}
}

func TestConnectionDetailsDoNotGuessObservedPeer(t *testing.T) {
	p := &pane{hostname: "reported-host", profile: core.Profile{Name: "saved", Kind: "ssh", Host: "config-alias", Port: 2222, Username: "tester"}, hostObservation: core.HostObservation{Identity: core.HostIdentity{Hostname: "reported-host", Address: "192.0.2.67"}}}
	text := paneConnectionDetails(p)
	for _, part := range []string{"当前主机\n主机名：reported-host\n主机 IP：192.0.2.67", "初始 SSH 连接\n会话：saved\n配置地址：config-alias\n端口：2222", "实际连接 IP：未获取"} {
		if !strings.Contains(text, part) {
			t.Fatal(text)
		}
	}
}

func TestLocalIdentityInitializationAndReturnFromRemote(t *testing.T) {
	a := &App{}
	p := &pane{profile: core.Profile{Kind: "local", TitleMode: "shellIntegration"}}
	a.initializeHostname(p)
	if p.hostname == "" || p.hostname != p.hostObservation.Identity.Hostname {
		t.Fatal("local identity not initialized")
	}
	p.localNames.Aliases = append(p.localNames.Aliases, "previous-network-name")
	p.hostObservation.Identity = core.HostIdentity{Hostname: "remote-host", Address: "192.0.2.5"}
	a.useLocalHostIdentity(p)
	if p.hostObservation.Identity.Hostname == "remote-host" || p.localNames.Canonical("previous-network-name") != p.hostname {
		t.Fatal("return to local lost identity or earlier alias")
	}
}

func TestTabDetailsIncludeGroupAndAllPaneStates(t *testing.T) {
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	first := &pane{hostname: "one", status: "已连接", profile: core.Profile{Kind: "local"}}
	second := &pane{hostname: "two", status: "正在连接…", profile: core.Profile{Kind: "local"}}
	target := &tab{id: core.NewID(), active: first, root: &node{first: &node{pane: first}, second: &node{pane: second}}}
	a.tabs = []*tab{target}
	a.ensureGroups()
	details := a.tabConnectionDetails(target)
	for _, value := range []string{"标签组：", "分屏 1", "分屏 2", "主机名：one", "主机名：two", "连接状态：已连接", "连接状态：正在连接…"} {
		if !strings.Contains(details, value) {
			t.Fatal("missing tooltip detail", value)
		}
	}
}
