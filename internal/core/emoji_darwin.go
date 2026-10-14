//go:build darwin

package core

import (
	"github.com/ebitengine/purego"
	"sync"
)

var emojiProperties struct {
	once                sync.Once
	property            func(int32, int32) int8
	emoji, presentation int32
	mu                  sync.Mutex
	cache               map[rune]uint8
}

func emojiFlags(cp rune) uint8 {
	if cp < 0xa9 && cp != '#' && cp != '*' && (cp < '0' || cp > '9') {
		return 0
	}
	p := &emojiProperties
	p.once.Do(func() {
		h, e := purego.Dlopen("/usr/lib/libicucore.A.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if e != nil {
			return
		}
		a, e := purego.Dlsym(h, "u_getPropertyEnum")
		if e != nil {
			return
		}
		b, e := purego.Dlsym(h, "u_hasBinaryProperty")
		if e != nil {
			return
		}
		var get func(string) int32
		purego.RegisterFunc(&get, a)
		purego.RegisterFunc(&p.property, b)
		p.emoji = get("Emoji")
		p.presentation = get("Emoji_Presentation")
		p.cache = map[rune]uint8{}
	})
	if p.property == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if flags, ok := p.cache[cp]; ok {
		return flags
	}
	var flags uint8
	if p.property(int32(cp), p.emoji) != 0 {
		flags |= 1
	}
	if p.property(int32(cp), p.presentation) != 0 {
		flags |= 2
	}
	if len(p.cache) >= 4096 {
		clear(p.cache)
	}
	p.cache[cp] = flags
	return flags
}
func ColorEmojiFamily() string { return "Apple Color Emoji" }
