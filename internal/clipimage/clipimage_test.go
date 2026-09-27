package clipimage

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestDIB(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255}) // top left
	img.Set(1, 1, color.NRGBA{R: 1, G: 2, B: 3, A: 128})    // bottom right
	out := dib(img)
	if len(out) != 40+2*2*4 {
		t.Fatalf("len = %d", len(out))
	}
	if w, h, bpp := binary.LittleEndian.Uint32(out[4:]), binary.LittleEndian.Uint32(out[8:]), binary.LittleEndian.Uint16(out[14:]); w != 2 || h != 2 || bpp != 32 {
		t.Fatalf("header %d×%d %d bpp", w, h, bpp)
	}
	pixels := out[40:]
	// Bottom-up: the image's top row is the bitmap's last.
	if got := pixels[8:12]; got[0] != 30 || got[1] != 20 || got[2] != 10 || got[3] != 255 {
		t.Errorf("top left = %v", got)
	}
	if got := pixels[4:8]; got[0] != 3 || got[1] != 2 || got[2] != 1 || got[3] != 128 {
		t.Errorf("bottom right = %v", got)
	}
}
