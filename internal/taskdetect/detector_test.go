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
