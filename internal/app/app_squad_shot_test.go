package app

import (
	"image"
	"testing"
)

func TestFitWithin(t *testing.T) {
	for _, c := range []struct{ w, h, side, wantW, wantH int }{
		{2560, 1440, 640, 640, 360},
		{1440, 2560, 640, 360, 640},
		{500, 300, 640, 500, 300},
	} {
		b := fitWithin(image.NewRGBA(image.Rect(0, 0, c.w, c.h)), c.side).Bounds()
		if b.Dx() != c.wantW || b.Dy() != c.wantH {
			t.Errorf("fitWithin(%dx%d, %d) = %dx%d, want %dx%d", c.w, c.h, c.side, b.Dx(), b.Dy(), c.wantW, c.wantH)
		}
	}
}
