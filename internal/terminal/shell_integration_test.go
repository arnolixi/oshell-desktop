package terminal

import (
	"oshell-desktop/internal/core"
	"reflect"
	"testing"
)

func TestShellIntegrationFragmentedAndAlternate(t *testing.T) {
	loadLib(t)
	frame := "\x1b]777;OShellHost=1;outer-real|10.20.0.1|\x07"
	for split := 0; split <= len(frame); split++ {
		var identities []core.HostIdentity
		term, e := New(Options{Conn: newPipe(), OnShellIdentity: func(h core.HostIdentity) { identities = append(identities, h) }})
		if e != nil {
			t.Fatal(e)
		}
		term.Feed([]byte(frame[:split]))
		term.Feed([]byte(frame[split:]))
		if !reflect.DeepEqual(identities, []core.HostIdentity{{Hostname: "outer-real", Address: "10.20.0.1"}}) {
			t.Fatalf("split %d: %+v", split, identities)
		}
		if term.Text() != "" {
			t.Fatal("metadata appeared as text", term.Text())
		}
		term.Feed([]byte("\x1b[?1049h" + frame + "\x1b[?1049l"))
		if len(identities) != 1 {
			t.Fatal("alternate screen report accepted")
		}
		term.Feed([]byte("\x1bc" + frame))
		if len(identities) != 2 {
			t.Fatal("reset lost report receiver")
		}
		term.Close()
	}
}
func TestShellIntegrationObserverLeavesOutputUnchanged(t *testing.T) {
	inputs := []string{"before\x1b]777;OShellHost=1;host|10.0.0.1|\x1b\\after", "\x1bPignored\x1b]777;OShellHost=1;fake||\x07\x1b\\", "\x1b]777;OShellHost=1;bad\x18text"}
	for _, input := range inputs {
		var scanner integrationScanner
		out := ""
		n := 0
		for _, b := range []byte(input) {
			scanner.scan([]byte{b}, func(data []byte) { out += string(data) }, func(core.HostIdentity) { n++ })
		}
		if out != input {
			t.Fatal("observer changed bytes")
		}
		if input != inputs[0] && n != 0 {
			t.Fatal("report escaped opaque/cancelled sequence")
		}
	}
}
