package ocr

import (
	"image"
	"image/color"
	"testing"

	"github.com/local/mayak/internal/imaging"
)

// A short title at the left of a wide title bar is cut to the text with a
// margin; a crop drawn all over, or empty, is kept as it is.
func TestTrimToTextCutsAWideStripToItsTitle(t *testing.T) {
	strip := image.NewRGBA(image.Rect(0, 0, 973, 26))
	for y := 0; y < 26; y++ {
		for x := 0; x < 973; x++ {
			strip.Set(x, y, color.RGBA{14, 15, 16, 255})
		}
	}
	for y := 5; y < 21; y++ {
		for x := 4; x < 118; x++ {
			strip.Set(x, y, color.RGBA{230, 230, 225, 255})
		}
	}
	trimmed := trimToText(strip)
	if got := trimmed.Bounds(); got.Dx() != 118+12 || got.Dy() != 26 {
		t.Fatalf("trimmed to %v", got)
	}
	// The left margin is the crop's own edge, so the text keeps its place.
	if imaging.LumaOf(trimmed.At(4, 12)) < 200 || imaging.LumaOf(trimmed.At(125, 12)) > 40 {
		t.Fatal("text moved")
	}

	empty := image.NewRGBA(image.Rect(0, 0, 400, 26))
	if trimToText(empty) != image.Image(empty) {
		t.Fatal("an empty crop was changed")
	}
	full := image.NewRGBA(image.Rect(0, 0, 400, 26))
	for y := 0; y < 26; y++ {
		for x := 0; x < 400; x++ {
			if (x+y)%2 == 0 {
				full.Set(x, y, color.White)
			}
		}
	}
	if trimToText(full).Bounds() != full.Bounds() {
		t.Fatal("a crop drawn all over was cut")
	}
}
