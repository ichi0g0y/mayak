package taskdetect

import (
	"image"
	"image/jpeg"
	"image/png"
	"os"

	"github.com/local/mayak/internal/imaging"
	"github.com/local/mayak/internal/screenscale"
)

type Rect struct{ X, Y, W, H int }
type Preset struct {
	Width, Height                                int
	Anchor, CharacterAnchor, RaidCharacterAnchor Rect
	// TasksTabs are the character screen's Tasks tab since EFT 1.2.0.0, out
	// of a raid and in one, right of where they were (CharacterAnchor,
	// RaidCharacterAnchor): a Prestige tab came, and out of a raid a
	// Customization tab too.
	TasksTabs                    []TasksTab
	LeftPanel, RightPanel, Title Rect
	// StoryAnchor is the "Story" tab of the character's Tasks screen, lit
	// when a story chapter is shown; StoryTitle is the chapter's name under
	// the "Chapter" label, at a fixed place (the chapter page has no list).
	StoryAnchor, StoryTitle Rect
	// SearchBox is the Tasks screen's search box, right of the sub-tabs;
	// SubTabGap the empty bar between them.
	SearchBox, SubTabGap Rect
	MinScore             float64
}

var Preset2560 = Preset{Width: 2560, Height: 1440, Anchor: Rect{150, 18, 240, 55}, CharacterAnchor: Rect{1430, 5, 300, 60}, RaidCharacterAnchor: Rect{1170, 5, 260, 60}, TasksTabs: []TasksTab{{Tab: Rect{1540, 5, 240, 60}, Neighbor: Rect{1290, 5, 200, 60}}, {Tab: Rect{1295, 5, 130, 60}, Neighbor: Rect{1040, 5, 200, 60}}}, LeftPanel: Rect{10, 410, 740, 950}, RightPanel: Rect{770, 410, 1750, 950}, Title: Rect{810, 315, 720, 85}, StoryAnchor: Rect{28, 66, 228, 52}, StoryTitle: Rect{224, 211, 620, 70}, SearchBox: Rect{1250, 74, 400, 30}, SubTabGap: Rect{870, 74, 300, 30}, MinScore: .42}

// storyTabBright is the share of bright pixels the lit "Story" tab has at
// least (.87 measured; the "Side" list lit next to it gives it .06–.14).
const storyTabBright = .6

// With the Story tab lit, a chapter counts as a Tasks screen when the tab
// carries its label (storyTabEdges; .15 measured) and the character screen's
// Tasks tab is lit at least storyCharacterTab (.29 and .47 measured; a
// raid's inventory has .06–.09 there). A chapter with a large picture has
// few panel edges and scores just under MinScore otherwise.
const (
	storyTabEdges     = .08
	storyCharacterTab = .2
)

// subTabStoryEnd is where a run starting further left is the Story sub-tab
// (it starts at about 30; Side at 220 in English, 290 in Japanese).
const subTabStoryEnd = 150

// searchBoxDark and subTabGapDark are the shares of dark pixels the Tasks
// screen's search box and the bar before it have at least (.81 and 1 seen);
// a sky behind trees has fewer.
const (
	searchBoxDark = .7
	subTabGapDark = .9
)

// subTabTraderBright is the trader screen's anchor brightness under which the
// sub-tabs count: a trader's screen has its portraits where they would be.
const subTabTraderBright = .3

// subTabLit finds the Tasks screen's lit Side or Operational sub-tab (their
// places differ by language) in the row under the tabs: one run of mostly
// bright columns as wide as a button, not at Story's place first in the row,
// the rest of the row dark. It returns the run's brightness. A bright sky
// lights more of it.
func subTabLit(px imaging.Pixels) (float64, bool) {
	const x0, x1, y0, y1 = 20, 820, 72, 108
	lit := make([]bool, 0, (x1-x0)/2)
	litCount := 0
	for x := x0; x < x1; x += 2 {
		bright, total := 0, 0
		for y := y0; y < y1; y += 4 {
			if px.Luma(x, y) > 155 {
				bright++
			}
			total++
		}
		on := float64(bright)/float64(total) >= .6
		lit = append(lit, on)
		if on {
			litCount++
		}
	}
	// Runs of lit columns, gaps of a few letters' width closed.
	type run struct{ start, end, count int }
	var runs []run
	for i := 0; i < len(lit); i++ {
		if !lit[i] {
			continue
		}
		r := run{start: i, end: i, count: 1}
		for j := i + 1; j < len(lit) && j-r.end <= 8; j++ {
			if lit[j] {
				r.end, r.count = j, r.count+1
			}
		}
		runs = append(runs, r)
		i = r.end
	}
	var wide []run
	for _, r := range runs {
		if width := (r.end - r.start + 1) * 2; width >= 100 {
			wide = append(wide, r)
		}
	}
	if len(wide) != 1 {
		return 0, false
	}
	r := wide[0]
	width := (r.end - r.start + 1) * 2
	if width > 300 || x0+r.start*2 < subTabStoryEnd || litCount-r.count > len(lit)/10 {
		return 0, false
	}
	return float64(r.count) / float64(r.end-r.start+1), true
}

// TasksTab is where the Tasks tab is lit and, Neighbor, the Map tab left of
// it, dark then; a raid's bright sky lights both.
type TasksTab struct{ Tab, Neighbor Rect }

// neighborBright is the share of bright pixels the Map tab next to a lit
// Tasks tab stays under (unlit tabs are dark).
const neighborBright = .3

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
	px := imaging.Of(img)
	left := stats(px, p.LeftPanel)
	right := stats(px, p.RightPanel)
	anchor := stats(px, p.Anchor)
	characterAnchor := stats(px, p.CharacterAnchor)
	raidCharacterAnchor := stats(px, p.RaidCharacterAnchor)
	traderScore := imaging.Clamp01(anchor.bright*.72 + (left.edges+right.edges)*1.2 + (left.dark+right.dark)*.08)
	characterScore := imaging.Clamp01(characterAnchor.bright*.9 + (left.edges+right.edges)*.8 + (left.dark+right.dark)*.05)
	raidCharacterScore := imaging.Clamp01(raidCharacterAnchor.bright*.9 + (left.edges+right.edges)*.8 + (left.dark+right.dark)*.05)
	characterTabBright := characterAnchor.bright
	if raidCharacterScore > characterScore {
		characterScore = raidCharacterScore
		characterTabBright = raidCharacterAnchor.bright
	}
	// Story, Side and Operational sit under the tabs whatever tabs a mode or
	// version has: Side or Operational lit, with the bar dark beside them and
	// the search box dark, says the Tasks screen too (subTabLit). Story is left to the tabs
	// above: a raid's inventory can light its place.
	if anchor.bright < subTabTraderBright && stats(px, p.SearchBox).dark >= searchBoxDark && stats(px, p.SubTabGap).dark >= subTabGapDark {
		if lit, ok := subTabLit(px); ok {
			if tabScore := imaging.Clamp01(lit*.9 + (left.edges+right.edges)*.8 + (left.dark+right.dark)*.05); tabScore > characterScore {
				characterScore = tabScore
				characterTabBright = lit
			}
		}
	}
	for _, tab := range p.TasksTabs {
		lit := stats(px, tab.Tab)
		if tabScore := imaging.Clamp01(lit.bright*.9 + (left.edges+right.edges)*.8 + (left.dark+right.dark)*.05); tabScore > characterScore && stats(px, tab.Neighbor).bright < neighborBright {
			characterScore = tabScore
			characterTabBright = lit.bright
		}
	}
	score, layout, cropRect := traderScore, "trader-tasks", p.Title
	if characterScore > traderScore {
		score, layout, cropRect = characterScore, "character-tasks", selectedCharacterTitle(px)
		// The Story tab shows one chapter, its name at a fixed place, rather
		// than a list with a selected row.
		if story := stats(px, p.StoryAnchor); story.bright >= storyTabBright {
			layout, cropRect = "story-tasks", storyTitle(px, p.StoryTitle)
			if story.edges >= storyTabEdges && characterTabBright >= storyCharacterTab {
				score = max(score, p.MinScore)
			}
		}
	}
	isTasks := score >= p.MinScore
	result := Result{IsTasks: isTasks, Score: score, TraderScore: traderScore, CharacterScore: characterScore, CharacterTabBright: characterTabBright, Layout: layout, CropRect: cropRect}
	if isTasks {
		result.Crop = imaging.Crop(text, cropRect.X, cropRect.Y, cropRect.W, cropRect.H)
		result.CropDataURL, _ = imaging.PNGDataURL(result.Crop)
	}
	return result, nil
}

func selectedCharacterTitle(px imaging.Pixels) Rect {
	// Search only the task-name column. Location, progress, objective and reward
	// panels can all contain brighter pixels than the selected row, but the
	// selected row is the only broad light band in this column. Average a band
	// instead of choosing a single scanline so text and item icons cannot win.
	const x0, x1, y0, y1 = 300, 800, 150, 1280
	averages := make([]float64, 0, (y1-y0)/2)
	for y := y0; y < y1; y += 2 {
		var sum, count uint32
		for x := x0; x < x1; x += 8 {
			sum += uint32(px.Luma(x, y))
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
	// The title is cut around its own line of letters: the band, avoiding
	// their dark strokes, can settle above or below it, more so in the taller
	// rows since EFT 1.2.0.0.
	centerY := titleLine(px, y0+bestIndex*2)
	y := centerY - 48
	if y < y0 {
		y = y0
	}
	if y+96 > px.Height() {
		y = px.Height() - 96
	}
	return Rect{X: x0, Y: y, W: nameColumnEnd(px, y, x1) - x0, H: 96}
}

// titleLine returns the middle of the selected row's line of letters near y:
// lines with a few dark pixels (dark letters on the light row), not many (a
// separator, or an unselected row's dark background); the run of them
// nearest y that is as high as a line of text. Without one it returns y.
func titleLine(px imaging.Pixels, y int) int {
	const x0, x1, reach = 360, 800, 80
	text := func(yy int) bool {
		if yy < 0 || yy >= px.Height() {
			return false
		}
		dark, total := 0, 0
		for x := x0; x < x1; x += 4 {
			if px.Luma(x, yy) < 90 {
				dark++
			}
			total++
		}
		share := float64(dark) / float64(total)
		return share >= .03 && share <= .45
	}
	best, bestDistance := y, reach+1
	for start := y - reach; start <= y+reach; {
		if !text(start) {
			start++
			continue
		}
		end, gap := start, 0
		for yy := start + 1; yy <= y+reach+40 && gap <= 4; yy++ {
			if text(yy) {
				end, gap = yy, 0
			} else {
				gap++
			}
		}
		if height := end - start + 1; height >= 12 && height <= 50 {
			middle := (start + end) / 2
			if distance := imaging.Abs(middle - y); distance < bestDistance {
				best, bestDistance = middle, distance
			}
		}
		start = end + 1
	}
	return best
}

// nameColumnEnd returns the separator line right of the task-name column in
// the selected row, so long names are not cut off, or fallback when there is
// none. The column is wider than the band searched for the row.
func nameColumnEnd(px imaging.Pixels, y, fallback int) int {
	for x := fallback; x < 1300 && x < px.Width(); x++ {
		dark := 0
		for yy := y + 8; yy < y+88; yy++ {
			if px.Luma(x, yy) < 60 {
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

func stats(px imaging.Pixels, r Rect) metrics {
	step := 4
	var dark, bright, edges, total int
	for y := r.Y; y < r.Y+r.H; y += step {
		var prev uint32
		for x := r.X; x < r.X+r.W; x += step {
			v := uint32(px.Luma(x, y))
			if v < 95 {
				dark++
			}
			if v > 155 {
				bright++
			}
			if prev > 0 && imaging.Abs(int(v)-int(prev)) > 38 {
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
func init() {
	image.RegisterFormat("png", "\x89PNG", png.Decode, png.DecodeConfig)
	image.RegisterFormat("jpeg", "\xff\xd8", jpeg.Decode, jpeg.DecodeConfig)
}

// storyTitle narrows the chapter's title area to its text: the chapter's
// picture starts to the right of the name and would be read as letters. The
// name is near-white; it ends at the first gap of storyTitleGap columns
// without such pixels (a space between words is about 16 px).
const storyTitleGap = 48

func storyTitle(px imaging.Pixels, r Rect) Rect {
	textAt := func(x int) bool {
		n := 0
		for y := r.Y; y < r.Y+r.H; y++ {
			cr, cg, cb := px.RGB(x, y)
			hi, lo := max(cr, cg, cb), min(cr, cg, cb)
			if hi > 175 && int(hi)-int(lo) < 45 {
				if n++; n >= 2 {
					return true
				}
			}
		}
		return false
	}
	last := -1
	for x := r.X; x < r.X+r.W; x++ {
		if textAt(x) {
			last = x
		} else if last >= 0 && x-last > storyTitleGap {
			break
		}
	}
	if last < 0 {
		return r
	}
	width := min(last-r.X+16, r.W)
	if width < 60 {
		return r
	}
	return Rect{r.X, r.Y, width, r.H}
}
