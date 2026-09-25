package screenscale

import (
	"image"
	"image/color"
	"testing"
)

func TestTo1440p(t *testing.T) {
	native := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	if got, err := To1440p(native); err != nil || got != image.Image(native) {
		t.Fatalf("2560×1440 should pass unchanged: %v", err)
	}
	// A 1080p screenshot is scaled; a white square keeps its relative place.
	small := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 540; y < 600; y++ {
		for x := 960; x < 1020; x++ {
			small.Set(x, y, color.White)
		}
	}
	got, err := To1440p(small)
	if err != nil {
		t.Fatal(err)
	}
	if b := got.Bounds(); b.Dx() != 2560 || b.Dy() != 1440 {
		t.Fatalf("scaled to %v", b)
	}
	if r, _, _, _ := got.At(1320, 760).RGBA(); r < 0xf000 {
		t.Fatal("the square did not scale with the screen")
	}
	for _, size := range []image.Rectangle{image.Rect(0, 0, 3440, 1440), image.Rect(0, 0, 1600, 1200)} {
		if _, err := To1440p(image.NewRGBA(size)); err != ErrUnsupported {
			t.Fatalf("%v: %v", size, err)
		}
	}
	if _, err := To1440p(image.NewRGBA(image.Rect(0, 0, 1366, 768))); err != nil {
		t.Fatalf("1366×768 is 16:9 enough: %v", err)
	}
}
