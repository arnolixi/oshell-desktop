package desktop

import (
	_ "embed"
	"github.com/egoist/mygo/ui"
	"sync"
)

//go:embed assets/brand.png
var brandPNG []byte
var brandImage = sync.OnceValue(func() *ui.Bitmap {
	b, err := ui.DecodeBitmap(brandPNG)
	if err != nil {
		panic(err)
	}
	return b
})
