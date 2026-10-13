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
	magnifier(img, left, top, color.RGBA{R: 200, G: 210, B: 215, A: 255})
	return img
}

// magnifier draws an item window's icon left of its title: a ring up to the
// right and its handle down to the left.
func magnifier(img *image.RGBA, left, top int, c color.Color) {
	cx, cy := left+24, top+13
	for y := cy - 6; y <= cy+6; y++ {
		for x := cx - 6; x <= cx+6; x++ {
			d := (x-cx)*(x-cx) + (y-cy)*(y-cy)
			if d >= 16 && d <= 36 {
				img.Set(x, y, c)
			}
		}
	}
	for i := 0; i < 7; i++ {
		img.Set(cx-5-i, cy+5+i, c)
		img.Set(cx-4-i, cy+5+i, c)
	}
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

// A stash row can look like a header (a grid line along its top, a reddish
// figurine at its end for the close button, the grid's lines down past it);
// its items' grey pictures fill the "title bar", which a window's never has.
func TestAnalyzeRejectsAStashRow(t *testing.T) {
	const left, top, width = 1686, 438, 843
	img := syntheticInspectWindow(left, top, width)
	grey := color.RGBA{R: 70, G: 72, B: 74, A: 255}
	for y := top + 6; y < top+46; y++ {
		for x := left + 30; x < left+width-60; x++ {
			if (x/40)%2 == 0 {
				img.Set(x, y, grey)
			}
		}
	}
	if result, _ := Analyze(img); result.IsItem {
		t.Fatalf("stash row taken for a window: %+v", result)
	}
}

// A header running past the window (a grey line behind it at the height of
// its top border) still finds the side borders a little inside its ends.
func TestWindowHeightLooksInsideTheHeadersEnds(t *testing.T) {
	const left, top, width = 746, 305, 1068
	img := syntheticInspectWindow(left, top, width)
	px := imaging.Of(img)
	if got := windowHeight(px, Rect{X: left - 5, Y: top, W: width + 18, H: 48}); got < minWindowHeight {
		t.Fatalf("height = %d", got)
	}
}

// EFT's item search has the same header with its magnifier the other way
// round (its handle down to the right), the sorting table a grid: neither is
// an item's window.
func TestAnalyzeTellsTheItemWindowByItsIcon(t *testing.T) {
	const left, top, width = 747, 150, 1065
	if result, _ := Analyze(syntheticInspectWindow(left, top, width)); !result.IsItem {
		t.Fatalf("an item's window: %+v", result)
	}
	search := syntheticInspectWindow(left, top, width)
	for y := top + 4; y < top+30; y++ {
		for x := left + 6; x < left+36; x++ {
			search.Set(x, y, color.Black)
		}
	}
	// Mirrored: the ring up to the left, the handle down to the right.
	c := color.RGBA{R: 220, G: 210, B: 180, A: 255}
	cx, cy := left+18, top+13
	for y := cy - 6; y <= cy+6; y++ {
		for x := cx - 6; x <= cx+6; x++ {
			if d := (x-cx)*(x-cx) + (y-cy)*(y-cy); d >= 16 && d <= 36 {
				search.Set(x, y, c)
			}
		}
	}
	for i := 0; i < 7; i++ {
		search.Set(cx+5+i, cy+5+i, c)
		search.Set(cx+4+i, cy+5+i, c)
	}
	if result, _ := Analyze(search); result.IsItem {
		t.Fatalf("the item search taken for an item's window: %+v", result)
	}
	grid := syntheticInspectWindow(left, top, width)
	for y := top + 4; y < top+30; y++ {
		for x := left + 6; x < left+36; x++ {
			grid.Set(x, y, color.Black)
		}
	}
	for _, gy := range []int{8, 15, 22} {
		for _, gx := range []int{12, 19, 26} {
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					grid.Set(left+gx+x, top+gy+y, c)
				}
			}
		}
	}
	if result, _ := Analyze(grid); result.IsItem {
		t.Fatalf("the sorting table taken for an item's window: %+v", result)
	}
}
