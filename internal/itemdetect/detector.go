package itemdetect

import (
	"errors"
	"image"
	_ "image/png"
	"os"

	"github.com/local/mayak/internal/imaging"
	"github.com/local/mayak/internal/screenscale"
)

type Rect struct{ X, Y, W, H int }

type Result struct {
	IsItem bool
	// Offer is true for the flea market's "Offer creation" window, whose
	// item name is inside the window instead of in its title.
	Offer       bool
	Score       float64
	WindowRect  Rect
	CropRect    Rect
	Crop        image.Image
	CropDataURL string
}

func AnalyzeFile(path string) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return Result{}, err
	}
	return Analyze(img)
}

// Analyze finds an item window in a 16:9 to 16:10 screenshot of any size;
// the rules below are measured at 2560 px wide (see internal/screenscale).
func Analyze(original image.Image) (Result, error) {
	img, err := screenscale.To1440p(original)
	if err != nil {
		return Result{}, err
	}
	px := imaging.Of(img)
	window, score := findInspectWindow(px)
	result := Result{IsItem: score >= .72, Score: score, WindowRect: window}
	if !result.IsItem {
		return result, nil
	}
	// The window may be dragged anywhere, but its title is always relative to
	// the top border. Keep only the title line: skip the magnifier icon on the
	// left (read as "P"), the close button, and the category line and weight
	// below the title.
	crop := Rect{X: window.X + 33, Y: window.Y + 2, W: window.W - 97, H: 26}
	if isOfferWindow(window) {
		// The offered item's name is under its picture in the OFFER panel.
		// The window can hang off the left edge, so measure from the right.
		result.Offer = true
		// Tall enough for Japanese glyphs, which rise above the Latin ones.
		crop = Rect{X: window.X + window.W - 682, Y: window.Y + 392, W: 600, H: 38}
	}
	if crop.W < 120 {
		return Result{}, errors.New("item inspection window is too narrow")
	}
	result.CropRect = crop
	// The title is read from a smoothly scaled copy (see screenscale).
	text, err := screenscale.Smooth1440p(original)
	if err != nil {
		return Result{}, err
	}
	result.Crop = imaging.Crop(text, crop.X, crop.Y, crop.W, crop.H)
	result.CropDataURL, _ = imaging.PNGDataURL(result.Crop)
	return result, nil
}

// The "Offer creation" window is 1602 px wide at 1440p; inspect windows
// are about 1068 px.
func isOfferWindow(r Rect) bool { return r.W >= 1560 && r.W <= 1640 }

// minWindowHeight is the least height of an inspect window at 1440p; the
// smallest (an item without description) is still well above it.
const minWindowHeight = 200

// findInspectWindow returns the frontmost item inspection window. Several can
// be open and overlap; their headers look the same whether focused or not, so
// the stacking order comes from occlusion: a window is behind another when
// that window's header is drawn over its body, and a window whose close button
// is hidden is behind something too.
func findInspectWindow(px imaging.Pixels) (Rect, float64) {
	headers := findHeaders(px)
	heights := make([]int, len(headers))
	for i, h := range headers {
		heights[i] = windowHeight(px, h.rect)
	}
	best, bestScore, bestCovers := Rect{}, 0.0, -1
	for i, a := range headers {
		// A grid line in the stash can look like a header, with a red item
		// (a Defibrillator) at its end as the "close button", but its border
		// stops after one row. Inspect windows are always much taller.
		if a.score < .72 || heights[i] < minWindowHeight {
			continue
		}
		covers := 0
		for j, b := range headers {
			// A line without a close button inside a's width is part of a's
			// own layout (the offer window's panels), not a window over it.
			inside := b.rect.X >= a.rect.X-8 && b.rect.X+b.rect.W <= a.rect.X+a.rect.W+8
			if i != j && !(b.score < .72 && inside) && covered(a.rect, heights[i], b.rect, heights[j]) {
				covers++
			}
		}
		// The least covered window wins; among equals the higher score, then
		// the one opened lower on screen (usually the newer one).
		if bestCovers < 0 || covers < bestCovers || covers == bestCovers && (a.score > bestScore || a.score == bestScore && a.rect.Y > best.Y) {
			best, bestScore, bestCovers = a.rect, a.score, covers
		}
	}
	return best, bestScore
}

// covered reports whether window b lies over window a: b's header is drawn
// inside a's body, or a's header stops exactly at b's side border (the rest
// of a's header runs underneath b).
func covered(a Rect, aHeight int, b Rect, bHeight int) bool {
	overlapsX := b.X < a.X+a.W-8 && b.X+b.W > a.X+8
	if b.Y > a.Y+10 && b.Y < a.Y+aHeight && overlapsX {
		return true
	}
	besideB := a.Y > b.Y && a.Y < b.Y+bHeight
	return besideB && (imaging.Abs(a.X-(b.X+b.W)) <= 8 || imaging.Abs(a.X+a.W-b.X) <= 8)
}

type header struct {
	rect Rect
	// score is the inspect-header confidence; below .72 the close button is
	// not visible, so the header only matters for occlusion.
	score float64
}

// findHeaders returns every inspect-window header on screen, once each.
func findHeaders(px imaging.Pixels) []header {
	var headers []header
	// Windows can be dragged up to the top bar, just under the screen's top.
	for y := 40; y < px.Height()-120; y++ {
		runStart := -1
		for x := 20; x < px.Width()-20; x++ {
			if isBorderAt(px, x, y) {
				if runStart < 0 {
					runStart = x
				}
				continue
			}
			if runStart >= 0 {
				if h, ok := headerAt(px, runStart, x-1, y); ok && !duplicate(headers, h) {
					headers = append(headers, h)
				}
				runStart = -1
			}
		}
	}
	return headers
}

func duplicate(headers []header, h header) bool {
	for i, other := range headers {
		if imaging.Abs(other.rect.Y-h.rect.Y) <= 6 && imaging.Abs(other.rect.X-h.rect.X) <= 12 && imaging.Abs(other.rect.W-h.rect.W) <= 24 {
			if h.score > other.score {
				headers[i].score = h.score
			}
			return true
		}
	}
	return false
}

// headerAt checks a border run as the top of an inspect window: a long
// neutral line over a dark title bar. The score rates its close button.
func headerAt(px imaging.Pixels, left, right, top int) (header, bool) {
	if right-left+1 < 100 {
		return header{}, false
	}
	left = expandHeaderLeft(px, right, top)
	width := right - left + 1
	if width < 480 || width > 1700 || !darkBar(px, left, right, top) {
		return header{}, false
	}
	score := closeButtonScore(px, right, top, width)
	// A line of the same colour behind the window (an equipment grid) can
	// carry on from its top border: then the close button is a little left of
	// where the line ends, and the window ends there.
	for shift := 2; score < .72 && shift <= 120 && width-shift >= 480; shift += 2 {
		if candidate := closeButtonScore(px, right-shift, top, width-shift); candidate >= .72 {
			right, width, score = right-shift, width-shift, candidate
		}
	}
	// The same line can carry on to the left of the window (an equipment
	// slot's frame at the height of its top border): the window starts where
	// its side border runs down the title bar.
	if score >= .72 {
		left = windowLeft(px, left, right, top)
		width = right - left + 1
	}
	return header{rect: Rect{X: left, Y: top, W: width, H: 48}, score: score}, true
}

// windowLeft is the leftmost column of the header run left..right that is
// the window's side border: border pixels down the title bar's 48 rows (45
// of them at least, a scaled screenshot blurs a few) with the dark bar just
// inside it (an equipment slot's frame behind the window has a dimmer,
// broken line and a lit label inside). Without one (a window hanging off
// the screen's left edge) the run's start stands.
func windowLeft(px imaging.Pixels, left, right, top int) int {
	bottom := min(top+48, px.Height())
	dark := func(x0, x1 int) bool {
		sum, n := 0, 0
		for y := top + 6; y < top+46 && y < bottom; y++ {
			for x := x0; x < x1; x++ {
				sum += int(px.Luma(x, y))
				n++
			}
		}
		return n > 0 && sum/n < 30
	}
	for x := left; x <= right-480; x++ {
		rows := 0
		for y := top; y < bottom; y++ {
			if isBorderAt(px, x, y) || isBorderAt(px, x+1, y) {
				rows++
			}
		}
		// The columns right inside the border are dark too: a frame of the
		// border colour next to the window would pass on the wider block alone.
		if rows >= 45 && dark(x+2, x+4) && dark(x+4, x+12) {
			return x
		}
	}
	return left
}

func darkBar(px imaging.Pixels, left, right, top int) bool {
	sum, n := 0, 0
	for y := top + 8; y < top+40 && y < px.Height(); y += 4 {
		for x := left + (right-left)/4; x < right-(right-left)/4; x += 8 {
			sum += int(px.Luma(x, y))
			n++
		}
	}
	return n > 0 && sum/n < 60
}

// closeButtonScore combines the red close button and its bright X glyph at
// the right end of a header. Other red inventory cells lack the border.
func closeButtonScore(px imaging.Pixels, right, top, width int) float64 {
	red, bright, total := 0, 0, 0
	for y := top + 4; y < top+42 && y < px.Height(); y += 2 {
		for x := right - 44; x <= right-3; x += 2 {
			if x < 0 || x >= px.Width() {
				continue
			}
			r, g, b := px.RGB(x, y)
			if r > 45 && int(r) > int(g)*2 && int(r) > int(b)*3/2 {
				red++
			}
			if imaging.Luma(r, g, b) > 155 {
				bright++
			}
			total++
		}
	}
	if total == 0 {
		return 0
	}
	redRatio, brightRatio := float64(red)/float64(total), float64(bright)/float64(total)
	if redRatio < .10 || brightRatio < .012 {
		return 0
	}
	return min(1, redRatio*3.2+brightRatio*2.5+float64(width)/3200)
}

// windowHeight follows the window's side borders down from its header. The
// left one can be off screen, so the longer of the two counts.
func windowHeight(px imaging.Pixels, r Rect) int {
	// A scaled screenshot can put a side border a pixel or two off the
	// header's ends: look around them.
	height := 0
	for _, dx := range []int{-2, 0, 2} {
		height = max(height, borderHeight(px, r, r.X+dx, r.X+dx+1), borderHeight(px, r, r.X+r.W-1+dx, r.X+r.W-2+dx))
	}
	return height
}

func borderHeight(px imaging.Pixels, r Rect, x0, x1 int) int {
	gap, bottom := 0, r.Y+r.H
	for y := r.Y; y < px.Height(); y++ {
		if isBorderAt(px, x0, y) || isBorderAt(px, x1, y) {
			bottom, gap = y, 0
			continue
		}
		if gap++; gap > 4 {
			break
		}
	}
	return max(bottom-r.Y, r.H)
}

func expandHeaderLeft(px imaging.Pixels, right, top int) int {
	left, gap := right, 0
	for x := right; x >= 20 && right-x < 1700; x-- {
		if isBorderAt(px, x, top) {
			left = x
			gap = 0
			continue
		}
		gap++
		// Only tiny breaks: a wider one is where the header ends and some other
		// line of the same color (inventory, other windows) begins.
		if gap > 3 {
			break
		}
	}
	return left
}

func isBorderAt(px imaging.Pixels, x, y int) bool { return isBorderPixel(px.RGB(x, y)) }

func isBorderPixel(r, g, b uint8) bool {
	maxValue, minValue := max(r, g, b), min(r, g, b)
	luma := imaging.Luma(r, g, b)
	// The border is slightly translucent: over a blue item it reads as
	// (58,62,71), so allow a faint tint.
	// At 1080p it is drawn thinner and reads darker, (37,40,41).
	return maxValue-minValue <= 16 && luma >= 34 && luma <= 115
}
