package terminal

import (
	"oshell-desktop/internal/core"
	"strings"
)

// This bounded observer follows OSC/string framing but never consumes or edits
// terminal bytes. Complete-frame boundaries let the VT process preceding mode
// changes before checking whether a report arrived in the alternate screen.
type integrationScanner struct {
	state    byte
	data     []byte
	overflow bool
}

func (s *integrationScanner) scan(input []byte, write func([]byte), report func(core.HostIdentity)) {
	start := 0
	finish := func(end int) {
		write(input[start:end])
		start = end
		if !s.overflow && strings.HasPrefix(string(s.data), "777;") {
			if identity, ok := core.ParseShellIntegration(string(s.data[4:])); ok {
				report(identity)
			}
		}
		s.state = 0
		s.data = s.data[:0]
		s.overflow = false
	}
	appendByte := func(b byte) {
		if len(s.data) < 4128 {
			s.data = append(s.data, b)
		} else {
			s.overflow = true
		}
	}
	for i, b := range input {
		if b == 0x18 || b == 0x1a {
			s.state = 0
			s.data = s.data[:0]
			s.overflow = false
			continue
		}
		switch s.state {
		case 0:
			if b == 0x1b {
				s.state = 1
			}
		case 1:
			switch b {
			case ']':
				s.state = 2
				s.data = s.data[:0]
				s.overflow = false
			case 'P', 'X', '^', '_':
				s.state = 4
			case 0x1b:
			default:
				s.state = 0
			}
		case 2:
			switch b {
			case 7:
				finish(i + 1)
			case 0x1b:
				s.state = 3
			default:
				if b >= 0x20 {
					appendByte(b)
				}
			}
		case 3:
			if b == '\\' {
				finish(i + 1)
			} else {
				s.data = s.data[:0]
				s.overflow = false
				s.state = 0
				if b == 0x1b {
					s.state = 1
				}
			}
		case 4:
			if b == 0x1b {
				s.state = 5
			}
		case 5:
			if b == '\\' {
				s.state = 0
			} else if b != 0x1b {
				s.state = 4
			}
		}
	}
	if start < len(input) {
		write(input[start:])
	}
}

func (t *Terminal) BeginHostProbe(command string) bool {
	if !t.ManagedInputAvailable() {
		return false
	}
	t.mu.Lock()
	if t.term == nil || t.term.AltScreen() {
		t.mu.Unlock()
		return false
	}
	t.probeEcho = core.NewHostProbeEcho(command)
	t.mu.Unlock()
	t.Send([]byte(command + "\r"))
	return true
}
func (t *Terminal) EndHostProbe() {
	t.mu.Lock()
	var remaining []byte
	if t.probeEcho != nil {
		remaining = t.probeEcho.Flush()
		t.probeEcho = nil
	}
	t.mu.Unlock()
	if len(remaining) > 0 {
		t.write(remaining)
	}
}
