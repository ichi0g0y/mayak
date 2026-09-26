package imaging

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestBrightnessMatchesTheColourPackage(t *testing.T) {
	for _, c := range []color.RGBA{{0, 0, 0, 255}, {255, 255, 255, 255}, {58, 61, 62, 255}, {200, 30, 30, 255}, {17, 200, 99, 255}, {123, 45, 250, 255}} {
		if got, want := Gray(c.R, c.G, c.B), color.GrayModel.Convert(c).(color.Gray).Y; got != want {
			t.Fatalf("Gray(%v) = %d, GrayModel gives %d", c, got, want)
		}
		if got, want := LumaOf(c), Luma(c.R, c.G, c.B); got != want {
			t.Fatalf("LumaOf(%v) = %d, Luma gives %d", c, got, want)
		}
	}
	if Luma(255, 255, 255) != 255 || Luma(0, 0, 0) != 0 || Luma(100, 100, 100) != 100 {
		t.Fatal("luma of greys")
	}
}

func TestPixelsReadWhatAtReads(t *testing.T) {
	src := image.NewNRGBA(image.Rect(10, 20, 14, 23))
	src.Set(11, 21, color.NRGBA{200, 30, 30, 255})
	src.Set(13, 22, color.NRGBA{1, 2, 3, 255})
	p := Of(src)
	if p.Width() != 4 || p.Height() != 3 {
		t.Fatalf("size %d×%d", p.Width(), p.Height())
	}
	if r, g, b := p.RGB(1, 1); r != 200 || g != 30 || b != 30 {
		t.Fatalf("pixel (1,1) = %d %d %d", r, g, b)
	}
	if p.Luma(3, 2) != Luma(1, 2, 3) || p.Gray(3, 2) != Gray(1, 2, 3) {
		t.Fatal("luma and gray of a pixel")
	}
	// An RGBA image at the origin is used as it is.
	rgba := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if Of(rgba).RGBA != rgba {
		t.Fatal("an RGBA image was copied")
	}
	crop := Crop(src, 11, 21, 2, 1)
	if r, g, b := Of(crop).RGB(0, 0); crop.Bounds() != image.Rect(0, 0, 2, 1) || r != 200 || g != 30 || b != 30 {
		t.Fatalf("crop %v, first pixel %d %d %d", crop.Bounds(), r, g, b)
	}
	url, err := PNGDataURL(crop)
	if err != nil || !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("data URL %.30q %v", url, err)
	}
	if Abs(-3) != 3 || Abs(2) != 2 || Clamp01(1.5) != 1 || Clamp01(-1) != 0 || Clamp01(.5) != .5 {
		t.Fatal("abs or clamp")
	}
}
