package vt

import "testing"

func TestCustomWordBoundariesWithWideAndWrappedText(t *testing.T) {
	loadLib(t)
	for _, tc := range []struct{ boundaries, want string }{{" /", "目录"}, {" ", "目录/file.txt"}} {
		t.Run(tc.boundaries, func(t *testing.T) {
			term, err := NewTerminal(8, 4, Effects{})
			if err != nil {
				t.Fatal(err)
			}
			defer term.Free()
			term.Write([]byte("目录/file.txt end"))
			gesture, err := NewGesture(500e6, 5)
			if err != nil {
				t.Fatal(err)
			}
			defer gesture.Free(term)
			gesture.SetWordBoundaries(tc.boundaries)
			cell, _ := term.CellAt(0, 0)
			gesture.Press(term, cell, 2, 5, 600e6)
			gesture.Release(term, &cell)
			selection, ok := gesture.Press(term, cell, 2, 5, 700e6)
			if !ok {
				t.Fatal("no selection")
			}
			term.SetSelection(&selection)
			if text, _ := term.SelectionText(); text != tc.want {
				t.Fatalf("got %q want %q", text, tc.want)
			}
		})
	}
}
