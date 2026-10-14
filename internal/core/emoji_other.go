//go:build !darwin

package core

// Explicit emoji-font preference is currently implemented for the macOS
// acceptance target; other platforms keep their native font fallback.
func emojiFlags(rune) uint8    { return 0 }
func ColorEmojiFamily() string { return "" }
