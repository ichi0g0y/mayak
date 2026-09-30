// Package profiledetect finds the character's Overall screen in a
// screenshot (EFT's character screen, its first tab) and cuts out what its
// left panel shows, for OCR: the level, the small stats beside it (raids,
// kills, leave rate, survival rate, K/D, time online), and the rows of text
// under the character (the nickname, then the
// experience). Those rows sit lower in a raid than in the menu, and the
// badge before the experience depends on the game's edition, so they are
// found as rows of text rather than at a place. The right side changes with
// its tabs and scrolling and is not read.
//
// Places are measured on 2560×1440 screenshots (internal/screenscale).
package profiledetect

import (
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/local/mayak/internal/screenscale"
	xdraw "golang.org/x/image/draw"
)

// Result is what a screenshot shows: whether it is the Overall screen, and
// its parts, cut from the screenshot scaled smoothly (for reading).
type Result struct {
	IsProfile bool
	// Level is the level's large number.
	Level image.Image
	// StatsLeft are the lines of the raids, kills and leave rate;
	// StatsRight those of the survival rate, K/D and time online (whose
	// number takes a fourth line in some languages). A line each, as OCR
	// reads a line more surely than a block.
	StatsLeft, StatsRight []image.Image
	// Rows are the rows of text under the character, top to bottom.
	Rows []image.Image
	// Panel is the whole left panel, for a picture of the character.
	Panel image.Image
}

// The places on the screen (2560×1440).
var (
	// The first tab of the character screen, lit when the Overall screen
	// shows, and the next one, dark then.
	tabLit  = image.Rect(20, 20, 250, 52)
	tabNext = image.Rect(300, 20, 560, 52)
	// The level's number, and the dark room to its right.
	level     = image.Rect(125, 90, 260, 180)
	levelInk  = image.Rect(130, 95, 255, 175)
	levelRoom = image.Rect(260, 95, 330, 175)
	// The faction's badge.
	badge = image.Rect(55, 245, 160, 370)
	// The stats beside the level, in two columns of lines.
	statsLeft  = image.Rect(585, 0, 682, 0)
	statsRight = image.Rect(682, 0, 848, 0)
	statLines  = [][2]int{{80, 112}, {112, 146}, {146, 180}, {178, 208}}
	// Where the rows under the character are looked for: between the
	// outfit's pictures (in the menu), below the character's boots.
	rows = image.Rect(190, 1130, 690, 1345)
	// The left panel.
	panel = image.Rect(0, 55, 848, 1345)
)

// The share of bright pixels each place has on the Overall screen (measured
// on Japanese and English screenshots, in the menu and in a raid: the lit
// tab .84–.86, the next .07–.08, the level .16, the room beside it 0, the
// badge .32), with room around them.
const (
	minTabLit    = .7
	maxTabNext   = .2
	minLevelInk  = .04
	maxLevelInk  = .45
	maxLevelRoom = .05
	minBadge     = .15
	maxBadge     = .65
)

// AnalyzeFile is Analyze for a screenshot file.
func AnalyzeFile(path string) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return Result{}, err
	}
	return Analyze(img)
}

// Analyze tells whether img is the Overall screen and cuts its parts out.
func Analyze(img image.Image) (Result, error) {
	sharp, err := screenscale.To1440p(img)
	if err != nil {
		return Result{}, err
	}
	if !isProfile(sharp) {
		return Result{}, nil
	}
	smooth, err := screenscale.Smooth1440p(img)
	if err != nil {
		return Result{}, err
	}
	out := Result{
		IsProfile: true,
		Level:     crop(smooth, level),
		Panel:     crop(smooth, panel),
	}
	for i, line := range statLines {
		if i < 3 {
			out.StatsLeft = append(out.StatsLeft, crop(smooth, image.Rect(statsLeft.Min.X, line[0], statsLeft.Max.X, line[1])))
		}
		out.StatsRight = append(out.StatsRight, crop(smooth, image.Rect(statsRight.Min.X, line[0], statsRight.Max.X, line[1])))
	}
	for _, band := range textRows(smooth, rows) {
		out.Rows = append(out.Rows, crop(smooth, band))
	}
	return out, nil
}

func isProfile(img image.Image) bool {
	lit, next := bright(img, tabLit), bright(img, tabNext)
	ink, room, mark := bright(img, levelInk), bright(img, levelRoom), bright(img, badge)
	return lit >= minTabLit && next <= maxTabNext &&
		ink >= minLevelInk && ink <= maxLevelInk && room <= maxLevelRoom &&
		mark >= minBadge && mark <= maxBadge
}

// bright is the share of r's pixels brighter than the interface's dark
// background and grey labels.
func bright(img image.Image, r image.Rectangle) float64 {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return 0
	}
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if luma(img, x, y) > 170 {
				n++
			}
		}
	}
	return float64(n) / float64(r.Dx()*r.Dy())
}

func luma(img image.Image, x, y int) int {
	r, g, b, _ := img.At(x, y).RGBA()
	return int(r>>8+g>>8+b>>8) / 3
}

// textRows finds the rows of text in r: runs of lines with some bright
// pixels, as tall as a line of the panel's text, with a little room above
// and below each.
func textRows(img image.Image, r image.Rectangle) []image.Rectangle {
	r = r.Intersect(img.Bounds())
	var out []image.Rectangle
	start, gap := -1, 0
	flush := func(end int) {
		if start >= 0 && end-start >= 12 && end-start <= 60 {
			out = append(out, image.Rect(r.Min.X, max(r.Min.Y, start-6), r.Max.X, min(r.Max.Y, end+6)))
		}
		start = -1
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		ink := 0
		for x := r.Min.X; x < r.Max.X; x++ {
			if luma(img, x, y) > 140 {
				ink++
			}
		}
		switch {
		case ink >= 6:
			if start < 0 {
				start = y
			}
			gap = 0
		case start >= 0:
			gap++
			if gap > 3 {
				flush(y - gap + 1)
			}
		}
	}
	if start >= 0 {
		flush(r.Max.Y)
	}
	return out
}

func crop(img image.Image, r image.Rectangle) image.Image {
	r = r.Intersect(img.Bounds())
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Src)
	return dst
}

// Ink makes the panel's light text black on white and twice as large, as
// OCR reads it best (the panel's dark, textured background otherwise hides
// digits from it).
func Ink(src image.Image) image.Image {
	b := src.Bounds()
	gray := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			v := uint8(255)
			if luma(src, b.Min.X+x, b.Min.Y+y) > 110 {
				v = 0
			}
			gray.Pix[y*gray.Stride+x] = v
		}
	}
	dst := image.NewGray(image.Rect(0, 0, b.Dx()*2, b.Dy()*2))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), gray, gray.Bounds(), xdraw.Src, nil)
	return dst
}
