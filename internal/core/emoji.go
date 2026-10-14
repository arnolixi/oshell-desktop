package core

// ColorEmojiCluster follows the source app's scalar-property rule, including
// explicit text presentation taking precedence over emoji presentation.
func ColorEmojiCluster(cluster []rune) bool {
	if len(cluster) == 0 {
		return false
	}
	flags := emojiFlags(cluster[0])
	if flags&1 == 0 {
		return false
	}
	presentation := flags&2 != 0
	for _, cp := range cluster {
		if cp == 0xfe0e {
			return false
		}
		if cp == 0xfe0f || cp == 0x20e3 {
			presentation = true
		}
	}
	return presentation
}
