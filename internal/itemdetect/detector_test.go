package itemdetect

import (
	"image"
	"image/color"
	"testing"

	"github.com/local/mayak/internal/imaging"
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

// An equipment slot's frame behind the window can continue its top border to
// the left; the window starts at its own side border, not at the frame's.
func TestAnalyzeStopsAtTheWindowsOwnLeftBorder(t *testing.T) {
	const left, top, width = 453, 196, 1068
	img := syntheticInspectWindow(left, top, width)
	border := color.RGBA{R: 58, G: 61, B: 62, A: 255}
	// The frame: a box of the border colour, lit inside (a label), whose top
	// edge joins the window's top border with a break of a pixel or two.
	for y := top; y < top+180; y++ {
		for x := 330; x < left; x++ {
			img.Set(x, y, color.RGBA{R: 70, G: 72, B: 74, A: 255})
		}
	}
	for x := 330; x < left-1; x++ {
		img.Set(x, top, border)
	}
	for y := top; y < top+180; y += 7 {
		img.Set(330, y, color.RGBA{R: 140, G: 140, B: 138, A: 255})
	}
	result, err := Analyze(img)
	if err != nil {
		t.Fatal(err)
	}
	// The frame's column next to the border counts as the border too (a
	// blurred border of a scaled screenshot must), so a pixel off is fine.
	if !result.IsItem || imaging.Abs(result.WindowRect.X-left) > 1 || imaging.Abs(result.WindowRect.W-width) > 1 {
		t.Fatalf("window rect: %+v (item=%v)", result.WindowRect, result.IsItem)
	}
}
