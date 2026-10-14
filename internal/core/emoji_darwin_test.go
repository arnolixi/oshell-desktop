//go:build darwin

package core

import "testing"

func TestEmojiPreferencePresentation(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{{"😀", true}, {"©", false}, {"©\ufe0f", true}, {"😀\ufe0e", false}, {"1", false}, {"1\ufe0f\u20e3", true}, {"1\ufe0e\u20e3", false}, {"👩‍💻", true}, {"中", false}, {"\ue0b0", false}, {"A", false}, {"", false}} {
		if got := ColorEmojiCluster([]rune(tc.text)); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.text, got, tc.want)
		}
	}
}
