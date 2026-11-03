package profiledetect

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	xdraw "golang.org/x/image/draw"
)

// badgePicture draws a stand-in for a Prestige's picture (the real ones are
// tarkov.dev's): a shield of one colour with a bright bar of another, on a
// transparent ground.
func badgePicture(shield, bar color.RGBA, barX int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 132, 132))
	draw.Draw(img, image.Rect(20, 16, 112, 116), &image.Uniform{C: shield}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(barX, 10, barX+24, 122), &image.Uniform{C: bar}, image.Point{}, draw.Src)
	return img
}

func TestPrestigeLevelTellsTheBadgeByItsPicture(t *testing.T) {
	silver := color.RGBA{R: 210, G: 215, B: 220, A: 255}
	icons := map[int]image.Image{
		1: badgePicture(color.RGBA{R: 120, G: 120, B: 125, A: 255}, silver, 54),
		2: badgePicture(color.RGBA{R: 170, G: 140, B: 70, A: 255}, silver, 30),
		3: badgePicture(color.RGBA{R: 120, G: 70, B: 50, A: 255}, silver, 78),
	}
	// The box as the screen shows it: the badge larger, on the dark ground.
	box := image.NewRGBA(image.Rect(0, 0, 165, 160))
	draw.Draw(box, box.Bounds(), &image.Uniform{C: prestigeBackground}, image.Point{}, draw.Src)
	xdraw.ApproxBiLinear.Scale(box, image.Rect(20, 10, 150, 150), icons[3], icons[3].Bounds(), draw.Over, nil)
	if got := PrestigeLevel(box, icons); got != 3 {
		t.Fatalf("Prestige = %d, want 3", got)
	}
	empty := image.NewRGBA(image.Rect(0, 0, 165, 160))
	draw.Draw(empty, empty.Bounds(), &image.Uniform{C: prestigeBackground}, image.Point{}, draw.Src)
	if got := PrestigeLevel(empty, icons); got != 0 {
		t.Fatalf("an empty box read Prestige %d", got)
	}
	// Two pictures alike: neither is told.
	twins := map[int]image.Image{3: icons[3], 4: icons[3]}
	if got := PrestigeLevel(box, twins); got != 0 {
		t.Fatalf("pictures alike read Prestige %d", got)
	}
	if got := PrestigeLevel(box, nil); got != 0 {
		t.Fatalf("no pictures read Prestige %d", got)
	}
}
