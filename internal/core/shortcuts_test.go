package core

import "testing"

func TestKeyboardBindingsAllPlatforms(t *testing.T) {
	for _, platform := range []string{"darwin", "windows", "linux"} {
		k := KeyboardShortcuts{}
		if e := k.Validate(platform); e != nil {
			t.Fatal(platform, e)
		}
		k.Overrides = map[string]ShortcutOverride{"newWindow": {Shortcut: &KeyboardShortcut{KeyCode: 3, Modifiers: 1}}}
		if platform != "darwin" {
			k.Overrides["newWindow"] = ShortcutOverride{Shortcut: &KeyboardShortcut{KeyCode: 3, Modifiers: 1}}
		}
		if e := k.Validate(platform); e == nil {
			t.Fatal("collision accepted", platform)
		}
	}
}

func TestNewDefaultsPreserveExistingOverrides(t *testing.T) {
	key := KeyboardShortcut{KeyCode: 31, Modifiers: 3}
	k := KeyboardShortcuts{Overrides: map[string]ShortcutOverride{"newWindow": {Shortcut: &key}}}
	if e := k.Validate("darwin"); e != nil {
		t.Fatal(e)
	}
	if len(k.Bindings("quickSwitch", "darwin")) != 0 || len(k.Bindings("newWindow", "darwin")) != 1 {
		t.Fatal("new shortcut stole existing binding")
	}
}
