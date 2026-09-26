package taskdetect

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"

	"github.com/local/mayak/internal/screenscale"
)

type Rect struct{ X, Y, W, H int }
type Preset struct {
	Width, Height                                int
	Anchor, CharacterAnchor, RaidCharacterAnchor Rect
	LeftPanel, RightPanel, Title                 Rect
	// StoryAnchor is the "Story" tab of the character's Tasks screen, lit
	// when a story chapter is shown; StoryTitle is the chapter's name under
	// the "Chapter" label, at a fixed place (the chapter page has no list).
	StoryAnchor, StoryTitle Rect
	MinScore                float64
}

var Preset2560 = Preset{Width: 2560, Height: 1440, Anchor: Rect{150, 18, 240, 55}, CharacterAnchor: Rect{1430, 5, 300, 60}, RaidCharacterAnchor: Rect{1170, 5, 260, 60}, LeftPanel: Rect{10, 410, 740, 950}, RightPanel: Rect{770, 410, 1750, 950}, Title: Rect{810, 315, 720, 85}, StoryAnchor: Rect{28, 66, 228, 52}, StoryTitle: Rect{224, 211, 620, 70}, MinScore: .42}

// storyTabBright is the share of bright pixels the lit "Story" tab has at
// least (.87 measured; the "Side" list lit next to it gives it .06–.14).
const storyTabBright = .6

type Result struct {
	IsTasks            bool
	Score              float64
	TraderScore        float64
	CharacterScore     float64
	CharacterTabBright float64
	Layout             string
	CropRect           Rect
	Crop               image.Image
	CropDataURL        string
}

func AnalyzeFile(path string, p Preset) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return Result{}, err
	}
	return Analyze(img, p)
}

// Analyze reads a task list from a screenshot. With the 2560×1440 preset, a
// 16:9 to 16:10 screenshot of any size is scaled to 2560 px wide first (see
// internal/screenscale).
func Analyze(img image.Image, p Preset) (Result, error) {
	// text is what the title is read from: a smoothly scaled copy.
	text := img
	if p.Width == screenscale.Width && p.Height == screenscale.Height {
		scaled, err := screenscale.To1440p(img)
		if err != nil {
			return Result{}, err
		}
		if scaled != img {
			if text, err = screenscale.Smooth1440p(img); err != nil {
				return Result{}, err
			}
		}
		img = scaled
	}
	b := img.Bounds()
	// A 16:10 screenshot is taller: the same layout with room below it.
	if b.Dx() != p.Width || b.Dy() < p.Height {
		return Result{}, screenscale.ErrUnsupported
	}
	left := stats(img, p.LeftPanel)
	right := stats(img, p.RightPanel)
	anchor := stats(img, p.Anchor)
	characterAnchor := stats(img, p.CharacterAnchor)
	raidCharacterAnchor := stats(img, p.RaidCharacterAnchor)
	traderScore := clamp(anchor.bright*.72 + (left.edges+right.edges)*1.2 + (left.dark+right.dark)*.08)
	characterScore := clamp(characterAnchor.bright*.9 + (left.edges+right.edges)*.8 + (left.dark+right.dark)*.05)
	raidCharacterScore := clamp(raidCharacterAnchor.bright*.9 + (left.edges+right.edges)*.8 + (left.dark+right.dark)*.05)
	characterTabBright := characterAnchor.bright
	if raidCharacterScore > characterScore {
		characterScore = raidCharacterScore
		characterTabBright = raidCharacterAnchor.bright
	}
	score, layout, cropRect := traderScore, "trader-tasks", p.Title
	if characterScore > traderScore {
		score, layout, cropRect = characterScore, "character-tasks", selectedCharacterTitle(img)
		// The Story tab shows one chapter, its name at a fixed place, rather
		// than a list with a selected row.
		if stats(img, p.StoryAnchor).bright >= storyTabBright {
			layout, cropRect = "story-tasks", p.StoryTitle
		}
	}
	isTasks := score >= p.MinScore
	result := Result{IsTasks: isTasks, Score: score, TraderScore: traderScore, CharacterScore: characterScore, CharacterTabBright: characterTabBright, Layout: layout, CropRect: cropRect}
	if isTasks {
		result.Crop = copyCrop(text, cropRect)
		result.CropDataURL, _ = dataURL(result.Crop)
	}
	return result, nil
}

func selectedCharacterTitle(img image.Image) Rect {
	// Search only the task-name column. Location, progress, objective and reward
	// panels can all contain brighter pixels than the selected row, but the
	// selected row is the only broad light band in this column. Average a band
	// instead of choosing a single scanline so text and item icons cannot win.
	const x0, x1, y0, y1 = 300, 800, 150, 1280
	averages := make([]float64, 0, (y1-y0)/2)
	for y := y0; y < y1; y += 2 {
		var sum, count uint32
		for x := x0; x < x1; x += 8 {
			sum += luma(img.At(x, y))
			count++
		}
		average := float64(sum) / float64(count)
		averages = append(averages, average)
	}

	const bandSamples = 32 // 64 source pixels; selected rows are about 100px high.
	windows := make([]float64, 0, len(averages)-bandSamples+1)
	bestWindow, best := 0, -1.0
	var bandTotal float64
	for i := 0; i < bandSamples; i++ {
		bandTotal += averages[i]
	}
	for start := 0; start+bandSamples <= len(averages); start++ {
		if start > 0 {
			bandTotal += averages[start+bandSamples-1] - averages[start-1]
		}
		average := bandTotal / bandSamples
		windows = append(windows, average)
		if average > best {
			best = average
			bestWindow = start
		}
	}
	first, last := bestWindow, bestWindow
	for first > 0 && windows[first-1] >= best*.995 {
		first--
	}
	for last+1 < len(windows) && windows[last+1] >= best*.995 {
		last++
	}
	bestIndex := (first+last)/2 + bandSamples/2
	centerY := y0 + bestIndex*2
	y := centerY - 48
	if y < y0 {
		y = y0
	}
	if y+96 > img.Bounds().Dy() {
		y = img.Bounds().Dy() - 96
	}
	return Rect{X: x0, Y: y, W: nameColumnEnd(img, y, x1) - x0, H: 96}
}

// nameColumnEnd returns the separator line right of the task-name column in
// the selected row, so long names are not cut off, or fallback when there is
// none. The column is wider than the band searched for the row.
func nameColumnEnd(img image.Image, y, fallback int) int {
	for x := fallback; x < 1300 && x < img.Bounds().Dx(); x++ {
		dark := 0
		for yy := y + 8; yy < y+88; yy++ {
			if luma(img.At(x, yy)) < 60 {
				dark++
			}
		}
		if dark*10 >= 80*8 {
			return x
		}
	}
	return fallback
}

type metrics struct{ dark, bright, edges float64 }

func stats(img image.Image, r Rect) metrics {
	step := 4
	var dark, bright, edges, total int
	for y := r.Y; y < r.Y+r.H; y += step {
		var prev uint32
		for x := r.X; x < r.X+r.W; x += step {
			v := luma(img.At(x, y))
			if v < 95 {
				dark++
			}
			if v > 155 {
				bright++
			}
			if prev > 0 && abs(int(v)-int(prev)) > 38 {
				edges++
			}
			prev = v
			total++
		}
	}
	if total == 0 {
		return metrics{}
	}
	return metrics{float64(dark) / float64(total), float64(bright) / float64(total), float64(edges) / float64(total)}
}
func luma(c color.Color) uint32 { r, g, b, _ := c.RGBA(); return (299*r + 587*g + 114*b) / 1000 >> 8 }
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
func copyCrop(src image.Image, r Rect) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, r.W, r.H))
	for y := 0; y < r.H; y++ {
		for x := 0; x < r.W; x++ {
			dst.Set(x, y, src.At(r.X+x, r.Y+y))
		}
	}
	return dst
}
func dataURL(img image.Image) (string, error) {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes()), nil
}
func init() {
	image.RegisterFormat("png", "\x89PNG", png.Decode, png.DecodeConfig)
	image.RegisterFormat("jpeg", "\xff\xd8", jpeg.Decode, jpeg.DecodeConfig)
}
