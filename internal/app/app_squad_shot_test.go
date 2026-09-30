package app

import (
	"image"
	"testing"
)

func TestFitWithin(t *testing.T) {
	for _, c := range []struct{ w, h, boxW, boxH, wantW, wantH int }{
		{2560, 1440, 640, 640, 640, 360},
		{1440, 2560, 640, 640, 360, 640},
		{500, 300, 640, 640, 500, 300},
		{2560, 1440, 1920, 1080, 1920, 1080},
		{2560, 1600, 1920, 1080, 1728, 1080},
		{2560, 1440, 1280, 720, 1280, 720},
	} {
		b := fitWithin(image.NewRGBA(image.Rect(0, 0, c.w, c.h)), c.boxW, c.boxH).Bounds()
		if b.Dx() != c.wantW || b.Dy() != c.wantH {
			t.Errorf("fitWithin(%dx%d, %dx%d) = %dx%d, want %dx%d", c.w, c.h, c.boxW, c.boxH, b.Dx(), b.Dy(), c.wantW, c.wantH)
		}
	}
}
