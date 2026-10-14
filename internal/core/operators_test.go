package core

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestQuickCommandCompatibilityAndInputBytes(t *testing.T) {
	var q QuickCommand
	if e := json.Unmarshal([]byte(`{"id":"test","name":"旧命令","text":"ls"}`), &q); e != nil {
		t.Fatal(e)
	}
	if !q.AppendReturn || q.Group != "常用" {
		t.Fatal("legacy command defaults lost", q)
	}
	q.AppendReturn = false
	q.Group = "部署"
	b, _ := json.Marshal(q)
	var copy QuickCommand
	json.Unmarshal(b, &copy)
	if copy != q {
		t.Fatal("roundtrip lost group/return")
	}
	for _, tc := range []struct {
		text   string
		append bool
		want   string
	}{{"ls", true, "ls\r"}, {"ls", false, "ls"}, {"a\r\nb\n", true, "a\rb\r"}, {"a\x1b[201~b", false, "a[201~b"}} {
		if got := string(ManagedInput(tc.text, tc.append)); got != tc.want {
			t.Fatalf("%q != %q", got, tc.want)
		}
	}
	q.Text = "bad\x00text"
	if q.Validate() == nil {
		t.Fatal("accepted null")
	}
}
func TestHighlightSetMigrationIsolationAndVariables(t *testing.T) {
	var cfg Config
	data := `{"version":2,"profiles":[],"settings":{"highlights":[{"id":"r","pattern":"${hostname}","regex":true,"foreground":"#123456","enabled":true}]}}`
	if e := json.Unmarshal([]byte(data), &cfg); e != nil {
		t.Fatal(e)
	}
	if len(cfg.HighlightSets) != 2 || cfg.Settings.HighlightSetID == StandardHighlightSetID {
		t.Fatal("legacy migration missing")
	}
	rules := cfg.ActiveHighlightRules("prod.example+test")
	if len(rules) != 1 {
		t.Fatal(rules)
	}
	re, e := regexp.Compile(rules[0].Pattern)
	if e != nil || !re.MatchString("prod.example+test") || re.MatchString("prodXexampletest") {
		t.Fatal("hostname not escaped", rules)
	}
	clone := cfg.Clone()
	clone.HighlightSets[1].Rules[0].Pattern = "changed"
	if cfg.HighlightSets[1].Rules[0].Pattern == "changed" {
		t.Fatal("shared mutable rules")
	}
	bytes, _ := json.Marshal(cfg)
	var round Config
	if e = json.Unmarshal(bytes, &round); e != nil || len(round.HighlightSets) != 2 {
		t.Fatal("migration repeated", e)
	}
	if e = json.Unmarshal([]byte(`{"settings":{"highlightSetID":null}}`), &round); e != nil || round.Settings.HighlightSetID != "" {
		t.Fatal("explicit disable ignored", e)
	}
	if got := round.ActiveHighlightRules("host"); len(got) != 0 {
		t.Fatal("disabled set active")
	}
}
func TestHighlightLimitsAndDisplayOnlyHostname(t *testing.T) {
	set := StandardHighlightSet()
	if e := set.Validate(); e != nil {
		t.Fatal(e)
	}
	set.Rules[0].Pattern = strings.Repeat("a", 513)
	if set.Validate() == nil {
		t.Fatal("oversized pattern accepted")
	}
	set = StandardHighlightSet()
	set.Rules[0].Pattern = "("
	if set.Validate() == nil {
		t.Fatal("invalid regex accepted")
	}
	for _, tc := range []struct{ title, want string }{{"user@prod.example:~", "prod.example"}, {"[user@host /tmp]", "host"}, {"user@bad\x1b[0m:~", ""}, {"not a host", ""}} {
		if got := HostnameFromTitle(tc.title); got != tc.want {
			t.Fatalf("%q: %q", tc.title, got)
		}
	}
}
