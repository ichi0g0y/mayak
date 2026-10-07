package taskdetect

import (
	"image"
	"image/color"
	"testing"

	"github.com/local/mayak/internal/imaging"
)

func TestRejectResolution(t *testing.T) {
	// 4:3 is not supported; 16:9 (and 16:10) of any size is scaled.
	if _, err := Analyze(image.NewRGBA(image.Rect(0, 0, 1600, 1200)), Preset2560); err == nil {
		t.Fatal("expected resolution error")
	}
	if _, err := Analyze(image.NewRGBA(image.Rect(0, 0, 1920, 1080)), Preset2560); err != nil {
		t.Fatalf("1080p should be scaled: %v", err)
	}
}
func TestDetectPanelStructure(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	for y := 0; y < 1440; y++ {
		for x := 0; x < 2560; x++ {
			img.Set(x, y, color.RGBA{18, 20, 19, 255})
		}
	}
	for _, r := range []Rect{Preset2560.LeftPanel, Preset2560.RightPanel} {
		for y := r.Y; y < r.Y+r.H; y += 20 {
			for x := r.X; x < r.X+r.W; x++ {
				img.Set(x, y, color.RGBA{180, 180, 170, 255})
			}
		}
	}
	for y := Preset2560.Anchor.Y; y < Preset2560.Anchor.Y+Preset2560.Anchor.H; y++ {
		for x := Preset2560.Anchor.X; x < Preset2560.Anchor.X+Preset2560.Anchor.W; x++ {
			img.Set(x, y, color.RGBA{210, 210, 205, 255})
		}
	}
	got, err := Analyze(img, Preset2560)
	if err != nil || !got.IsTasks {
		t.Fatalf("result=%+v err=%v", got, err)
	}
}

func TestSelectedCharacterTitleNearBottom(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	for y := 700; y < 800; y++ {
		for x := 320; x < 1040; x++ {
			img.Set(x, y, color.RGBA{170, 170, 165, 255})
		}
	}

	crop := selectedCharacterTitle(imaging.Of(img))
	if crop.Y > 710 || crop.Y+crop.H < 790 {
		t.Fatalf("selected row was clipped: %+v", crop)
	}
}

func TestSelectedCharacterTitleIgnoresBrighterThinContent(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	for y := 340; y < 450; y++ {
		for x := 300; x < 800; x++ {
			img.Set(x, y, color.RGBA{150, 150, 145, 255})
		}
	}
	// A bright line such as reward text used to beat the selected row.
	for y := 1218; y < 1226; y++ {
		for x := 300; x < 800; x++ {
			img.Set(x, y, color.RGBA{245, 245, 240, 255})
		}
	}

	crop := selectedCharacterTitle(imaging.Of(img))
	if crop.Y > 365 || crop.Y+crop.H < 425 {
		t.Fatalf("selected row was not chosen: %+v", crop)
	}
}

func TestDetectRaidCharacterTasksTab(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	for y := 0; y < 1440; y++ {
		for x := 0; x < 2560; x++ {
			img.Set(x, y, color.RGBA{18, 20, 19, 255})
		}
	}
	for y := Preset2560.RaidCharacterAnchor.Y; y < Preset2560.RaidCharacterAnchor.Y+Preset2560.RaidCharacterAnchor.H; y++ {
		for x := Preset2560.RaidCharacterAnchor.X; x < Preset2560.RaidCharacterAnchor.X+Preset2560.RaidCharacterAnchor.W; x++ {
			img.Set(x, y, color.RGBA{210, 210, 205, 255})
		}
	}
	for _, r := range []Rect{Preset2560.LeftPanel, Preset2560.RightPanel} {
		for y := r.Y; y < r.Y+r.H; y += 20 {
			for x := r.X; x < r.X+r.W; x++ {
				img.Set(x, y, color.RGBA{180, 180, 170, 255})
			}
		}
	}

	got, err := Analyze(img, Preset2560)
	if err != nil || !got.IsTasks || got.Layout != "character-tasks" {
		t.Fatalf("result=%+v err=%v", got, err)
	}
}

// The Story tab lit turns the character layout into the chapter page, whose
// name is read from a fixed place instead of a selected row.
func TestStoryTabSelectsTheChapterTitle(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	for y := 0; y < 1440; y++ {
		for x := 0; x < 2560; x++ {
			img.Set(x, y, color.RGBA{18, 20, 19, 255})
		}
	}
	for _, r := range []Rect{Preset2560.LeftPanel, Preset2560.RightPanel} {
		for y := r.Y; y < r.Y+r.H; y += 20 {
			for x := r.X; x < r.X+r.W; x++ {
				img.Set(x, y, color.RGBA{180, 180, 170, 255})
			}
		}
	}
	fill := func(r Rect) {
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				img.Set(x, y, color.RGBA{210, 210, 205, 255})
			}
		}
	}
	fill(Preset2560.CharacterAnchor)
	got, err := Analyze(img, Preset2560)
	if err != nil || !got.IsTasks || got.Layout != "character-tasks" {
		t.Fatalf("side list: result=%+v err=%v", got, err)
	}
	fill(Preset2560.StoryAnchor)
	got, err = Analyze(img, Preset2560)
	if err != nil || !got.IsTasks || got.Layout != "story-tasks" || got.CropRect != Preset2560.StoryTitle {
		t.Fatalf("story chapter: result=%+v err=%v", got, err)
	}
}

// A chapter with a large picture has few panel edges: with the Story tab lit
// (its label drawn) and the Tasks tab lit dimly, it still counts.
func TestStoryChapterWithAPictureCounts(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	for y := 0; y < 1440; y++ {
		for x := 0; x < 2560; x++ {
			img.Set(x, y, color.RGBA{18, 20, 19, 255})
		}
	}
	// The Tasks tab, a third of it lit.
	c := Preset2560.CharacterAnchor
	for y := c.Y; y < c.Y+c.H; y++ {
		for x := c.X; x < c.X+c.W/3; x++ {
			img.Set(x, y, color.RGBA{210, 210, 205, 255})
		}
	}
	got, err := Analyze(img, Preset2560)
	if err != nil || got.IsTasks {
		t.Fatalf("no Story tab: result=%+v err=%v", got, err)
	}
	// The lit Story tab with its label (dark strokes on it).
	s := Preset2560.StoryAnchor
	for y := s.Y; y < s.Y+s.H; y++ {
		for x := s.X; x < s.X+s.W; x++ {
			v := uint8(215)
			if y > s.Y+14 && y < s.Y+38 && (x/6)%3 == 0 {
				v = 40
			}
			img.Set(x, y, color.RGBA{v, v, v - 5, 255})
		}
	}
	got, err = Analyze(img, Preset2560)
	if err != nil || !got.IsTasks || got.Layout != "story-tasks" {
		t.Fatalf("chapter with a picture: result=%+v err=%v", got, err)
	}
	// The same Story tab with the Tasks tab dark (a raid's inventory) does not.
	for y := c.Y; y < c.Y+c.H; y++ {
		for x := c.X; x < c.X+c.W; x++ {
			img.Set(x, y, color.RGBA{18, 20, 19, 255})
		}
	}
	if got, _ = Analyze(img, Preset2560); got.IsTasks {
		t.Fatalf("dark Tasks tab counted: %+v", got)
	}
}

// The chapter's picture right of its name is left out of the title area.
func TestStoryTitleStopsBeforeThePicture(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	r := Preset2560.StoryTitle
	white := color.RGBA{232, 230, 220, 255}
	// Two words of "text" (a 16 px space between), then the picture 120 px on.
	for _, span := range [][2]int{{10, 120}, {136, 260}, {380, 620}} {
		for x := r.X + span[0]; x < r.X+span[1]; x += 3 {
			for y := r.Y + 20; y < r.Y+50; y++ {
				img.Set(x, y, white)
			}
		}
	}
	got := storyTitle(imaging.Of(img), r)
	if got.X != r.X || got.W < 260 || got.W > 300 {
		t.Fatalf("title area %+v, want it to end after the text (about 275 px)", got)
	}
}

// Since EFT 1.2.0.0 the Tasks tab of the character screen sits further
// right, out of a raid and in one: lit there, the list is the character's,
// not a trader's.
func TestMenuTasksTabSince120(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	for y := 0; y < 1440; y++ {
		for x := 0; x < 2560; x++ {
			img.Set(x, y, color.RGBA{18, 20, 19, 255})
		}
	}
	for _, r := range []Rect{Preset2560.LeftPanel, Preset2560.RightPanel} {
		for y := r.Y; y < r.Y+r.H; y += 20 {
			for x := r.X; x < r.X+r.W; x++ {
				img.Set(x, y, color.RGBA{180, 180, 170, 255})
			}
		}
	}
	fill := func(r Rect, c color.RGBA) {
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				img.Set(x, y, c)
			}
		}
	}
	lit, dark := color.RGBA{210, 210, 205, 255}, color.RGBA{18, 20, 19, 255}
	for i, tab := range Preset2560.TasksTabs {
		fill(tab.Tab, lit)
		got, err := Analyze(img, Preset2560)
		if err != nil || !got.IsTasks || got.Layout != "character-tasks" {
			t.Fatalf("tab %d: result=%+v err=%v", i, got, err)
		}
		fill(tab.Tab, dark)
	}
}

// Whatever tabs a mode has above, the lit Side sub-tab with the search box
// dark beside it says the Tasks screen; a sky lighting the whole row does not.
func TestSideSubTabSaysTasks(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	fill := func(r Rect, c color.RGBA) {
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				img.Set(x, y, c)
			}
		}
	}
	fill(Rect{0, 0, 2560, 1440}, color.RGBA{18, 20, 19, 255})
	for _, r := range []Rect{Preset2560.LeftPanel, Preset2560.RightPanel} {
		for y := r.Y; y < r.Y+r.H; y += 20 {
			fill(Rect{r.X, y, r.W, 1}, color.RGBA{180, 180, 170, 255})
		}
	}
	lit := color.RGBA{210, 210, 205, 255}
	fill(Rect{290, 62, 170, 54}, lit)
	got, err := Analyze(img, Preset2560)
	if err != nil || !got.IsTasks || got.Layout != "character-tasks" {
		t.Fatalf("Side lit: result=%+v err=%v", got, err)
	}
	fill(Rect{0, 62, 2560, 54}, lit)
	if got, _ := Analyze(img, Preset2560); got.Layout == "character-tasks" && got.IsTasks {
		t.Fatalf("a lit row: %+v", got)
	}
}
