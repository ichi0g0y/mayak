package itemdetect

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"

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
	window, score := findInspectWindow(img)
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
	result.Crop = copyCrop(text, crop)
	result.CropDataURL, _ = dataURL(result.Crop)
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
func findInspectWindow(img image.Image) (Rect, float64) {
	headers := findHeaders(img)
	heights := make([]int, len(headers))
	for i, h := range headers {
		heights[i] = windowHeight(img, h.rect)
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
	return besideB && (abs(a.X-(b.X+b.W)) <= 8 || abs(a.X+a.W-b.X) <= 8)
}

type header struct {
	rect Rect
	// score is the inspect-header confidence; below .72 the close button is
	// not visible, so the header only matters for occlusion.
	score float64
}

// findHeaders returns every inspect-window header on screen, once each.
func findHeaders(img image.Image) []header {
	bounds := img.Bounds()
	var headers []header
	// Windows can be dragged up to the top bar, just under the screen's top.
	for y := 40; y < bounds.Dy()-120; y++ {
		runStart := -1
		for x := 20; x < bounds.Dx()-20; x++ {
			if isBorderPixel(img.At(x, y)) {
				if runStart < 0 {
					runStart = x
				}
				continue
			}
			if runStart >= 0 {
				if h, ok := headerAt(img, runStart, x-1, y); ok && !duplicate(headers, h) {
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
		if abs(other.rect.Y-h.rect.Y) <= 6 && abs(other.rect.X-h.rect.X) <= 12 && abs(other.rect.W-h.rect.W) <= 24 {
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
func headerAt(img image.Image, left, right, top int) (header, bool) {
	if right-left+1 < 100 {
		return header{}, false
	}
	left = expandHeaderLeft(img, right, top)
	width := right - left + 1
	if width < 480 || width > 1700 || !darkBar(img, left, right, top) {
		return header{}, false
	}
	score := closeButtonScore(img, right, top, width)
	// A line of the same colour behind the window (an equipment grid) can
	// carry on from its top border: then the close button is a little left of
	// where the line ends, and the window ends there.
	for shift := 2; score < .72 && shift <= 120 && width-shift >= 480; shift += 2 {
		if candidate := closeButtonScore(img, right-shift, top, width-shift); candidate >= .72 {
			right, width, score = right-shift, width-shift, candidate
		}
	}
	return header{rect: Rect{X: left, Y: top, W: width, H: 48}, score: score}, true
}

func darkBar(img image.Image, left, right, top int) bool {
	sum, n := 0, 0
	for y := top + 8; y < top+40 && y < img.Bounds().Dy(); y += 4 {
		for x := left + (right-left)/4; x < right-(right-left)/4; x += 8 {
			r, g, b := rgb8(img.At(x, y))
			sum += int(luma8(r, g, b))
			n++
		}
	}
	return n > 0 && sum/n < 60
}

// closeButtonScore combines the red close button and its bright X glyph at
// the right end of a header. Other red inventory cells lack the border.
func closeButtonScore(img image.Image, right, top, width int) float64 {
	red, bright, total := 0, 0, 0
	for y := top + 4; y < top+42 && y < img.Bounds().Dy(); y += 2 {
		for x := right - 44; x <= right-3; x += 2 {
			if x < 0 || x >= img.Bounds().Dx() {
				continue
			}
			r, g, b := rgb8(img.At(x, y))
			if r > 45 && int(r) > int(g)*2 && int(r) > int(b)*3/2 {
				red++
			}
			if luma8(r, g, b) > 155 {
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
func windowHeight(img image.Image, r Rect) int {
	// A scaled screenshot can put a side border a pixel or two off the
	// header's ends: look around them.
	height := 0
	for _, dx := range []int{-2, 0, 2} {
		height = max(height, borderHeight(img, r, r.X+dx, r.X+dx+1), borderHeight(img, r, r.X+r.W-1+dx, r.X+r.W-2+dx))
	}
	return height
}

func borderHeight(img image.Image, r Rect, x0, x1 int) int {
	gap, bottom := 0, r.Y+r.H
	for y := r.Y; y < img.Bounds().Dy(); y++ {
		if isBorderPixel(img.At(x0, y)) || isBorderPixel(img.At(x1, y)) {
			bottom, gap = y, 0
			continue
		}
		if gap++; gap > 4 {
			break
		}
	}
	return max(bottom-r.Y, r.H)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func expandHeaderLeft(img image.Image, right, top int) int {
	left, gap := right, 0
	for x := right; x >= 20 && right-x < 1700; x-- {
		if isBorderPixel(img.At(x, top)) {
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

func isBorderPixel(c color.Color) bool {
	r, g, b := rgb8(c)
	maxValue, minValue := max(r, g, b), min(r, g, b)
	luma := luma8(r, g, b)
	// The border is slightly translucent: over a blue item it reads as
	// (58,62,71), so allow a faint tint.
	// At 1080p it is drawn thinner and reads darker, (37,40,41).
	return maxValue-minValue <= 16 && luma >= 34 && luma <= 115
}

func rgb8(c color.Color) (uint8, uint8, uint8) {
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func luma8(r, g, b uint8) uint8 {
	return uint8((299*uint32(r) + 587*uint32(g) + 114*uint32(b)) / 1000)
}

func copyCrop(src image.Image, rect Rect) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, rect.W, rect.H))
	for y := 0; y < rect.H; y++ {
		for x := 0; x < rect.W; x++ {
			dst.Set(x, y, src.At(rect.X+x, rect.Y+y))
		}
	}
	return dst
}

func dataURL(img image.Image) (string, error) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}
