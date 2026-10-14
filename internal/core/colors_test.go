package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBaselineColorPresetsAndImport(t *testing.T) {
	presets := ColorPresets()
	if len(presets) != 9 {
		t.Fatal(len(presets))
	}
	for _, p := range presets {
		if e := p.Validate(); e != nil {
			t.Fatal(p.Name, e)
		}
	}
	if presets[0].Background != "#11151A" || presets[0].Cursor != "#55E5CC" {
		t.Fatal("baseline colors changed")
	}
	data, _ := json.Marshal(presets[0])
	imported, e := ImportColors(data)
	if e != nil || len(imported) != 1 || imported[0].ID == presets[0].ID {
		t.Fatal(imported, e)
	}
	text := "[Example]\nbackground=ffffff\ntext=000000\n"
	for _, c := range []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"} {
		text += c + "=112233\n" + c + "(bold)=aabbcc\n"
	}
	text += "[Names]\ncount=1\nname0=Example\n"
	imported, e = ImportColors([]byte(text))
	if e != nil || len(imported) != 1 || imported[0].ANSI[8] != "#AABBCC" {
		t.Fatal(imported, e)
	}
	if _, e = ImportColors([]byte(strings.ReplaceAll(text, "text=000000\n", ""))); e == nil {
		t.Fatal("accepted incomplete Xshell scheme")
	}
}
