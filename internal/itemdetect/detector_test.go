package itemdetect

import (
	"image"
	"image/color"
	"testing"
)

func TestAnalyzeFindsMovedInspectWindow(t *testing.T) {
	for _, origin := range []image.Point{{340, 210}, {920, 620}} {
		img := syntheticInspectWindow(origin.X, origin.Y, 980)
		result, err := Analyze(img)
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsItem {
			t.Fatalf("inspect window at %v was not detected: %+v", origin, result)
		}
		if result.WindowRect.X != origin.X || result.WindowRect.Y != origin.Y || result.WindowRect.W != 980 {
			t.Fatalf("unexpected window rect at %v: %+v", origin, result.WindowRect)
		}
	}
}

func TestAnalyzeRejectsOrdinaryScreen(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	result, err := Analyze(img)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsItem {
		t.Fatalf("ordinary screen detected as item: %+v", result)
	}
}

func syntheticInspectWindow(left, top, width int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	border := color.RGBA{R: 58, G: 61, B: 62, A: 255}
	for x := left; x < left+width; x++ {
		img.Set(x, top, border)
	}
	// The left border runs down the whole window.
	for y := top; y < top+400; y++ {
		img.Set(left, y, border)
	}
	for y := top + 4; y < top+42; y++ {
		for x := left + width - 44; x < left+width-2; x++ {
			img.Set(x, y, color.RGBA{R: 75, G: 13, B: 13, A: 255})
		}
	}
	for i := 0; i < 24; i++ {
		x := left + width - 35 + i
		img.Set(x, top+10+i, color.White)
		img.Set(x, top+33-i, color.White)
	}
	return img
}

func TestCoveredWindows(t *testing.T) {
	back := Rect{X: 216, Y: 200, W: 1069, H: 48} // 700 px tall
	front := Rect{X: 488, Y: 585, W: 857, H: 48}
	beside := Rect{X: 1285, Y: 457, W: 817, H: 48}
	cases := []struct {
		name          string
		a, b          Rect
		aH, bH        int
		wantCoveredBy bool
	}{
		{"a header drawn inside the window's body", back, front, 700, 500, true},
		{"the window drawn over the other is not covered by it", front, back, 500, 700, false},
		{"a header cut off at another window's side border", beside, back, 430, 700, true},
		{"windows side by side without overlap", Rect{X: 100, Y: 200, W: 600, H: 48}, Rect{X: 900, Y: 300, W: 600, H: 48}, 500, 500, false},
	}
	for _, c := range cases {
		if got := covered(c.a, c.aH, c.b, c.bH); got != c.wantCoveredBy {
			t.Errorf("%s: covered = %v", c.name, got)
		}
	}
}

func TestAnalyzeReadsOfferWindowItemName(t *testing.T) {
	// The offer window is wider; its item name is inside, not in the title.
	result, err := Analyze(syntheticInspectWindow(300, 60, 1602))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsItem || !result.Offer {
		t.Fatalf("offer window was not detected: %+v", result)
	}
	if want := (Rect{X: 300 + 1602 - 682, Y: 60 + 392, W: 600, H: 38}); result.CropRect != want {
		t.Fatalf("crop = %+v, want %+v", result.CropRect, want)
	}
}
