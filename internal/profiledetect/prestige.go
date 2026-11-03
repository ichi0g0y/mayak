package profiledetect

import (
	"image"
	"image/color"
	"image/draw"

	xdraw "golang.org/x/image/draw"
)

// The Prestige's badge is a stylised number on a shield, one picture for
// each Prestige; OCR does not read it. It is told by the pictures tarkov.dev
// has of them (the catalog's prestige iconLink): both are cut to the bright
// part, made prestigeSide pixels square and compared colour by colour. The
// screen's is about 31 from its own picture and 46 from the next nearest
// (9 Overall screens of a Prestige 3, 2026-11-03).
const (
	prestigeSide = 24
	// maxPrestigeDiff is the most a badge differs from its picture (the mean
	// difference of a colour, 0 to 255); minPrestigeGap is the least the
	// next nearest picture differs more.
	maxPrestigeDiff = 40
	minPrestigeGap  = 8
	// prestigeInk is the least brightness of the badge's pixels, over the
	// box's dark background.
	prestigeInk = 70
	// The badges are a little narrower than tall (.83 to .91 of their
	// height, the screen's .83): a bright part of another shape is none.
	minPrestigeAspect, maxPrestigeAspect = .7, 1.0
)

// prestigeBackground is the box's background, under the pictures'
// transparent parts.
var prestigeBackground = color.RGBA{R: 24, G: 26, B: 27, A: 255}

// PrestigeLevel tells which Prestige the box's badge shows, by icons (the
// pictures by Prestige): 0 when none is near enough, or two are about as
// near (no box, or a badge with no picture).
func PrestigeLevel(box image.Image, icons map[int]image.Image) int {
	if box == nil || len(icons) == 0 {
		return 0
	}
	shown := prestigeSquare(box)
	if shown == nil {
		return 0
	}
	best, bestDiff, nextDiff := 0, 256.0, 256.0
	for level, icon := range icons {
		square := prestigeSquare(icon)
		if square == nil {
			continue
		}
		diff := prestigeDiff(shown, square)
		switch {
		case diff < bestDiff:
			best, bestDiff, nextDiff = level, diff, bestDiff
		case diff < nextDiff:
			nextDiff = diff
		}
	}
	if bestDiff > maxPrestigeDiff || nextDiff-bestDiff < minPrestigeGap {
		return 0
	}
	return best
}

// prestigeSquare puts img on the box's background, cuts it to its bright
// part and scales that to prestigeSide square; nil when nothing is bright.
func prestigeSquare(img image.Image) *image.RGBA {
	b := img.Bounds()
	flat := image.NewRGBA(b)
	draw.Draw(flat, b, &image.Uniform{C: prestigeBackground}, image.Point{}, draw.Src)
	draw.Draw(flat, b, img, b.Min, draw.Over)
	ink := image.Rectangle{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := flat.RGBAAt(x, y)
			if (int(c.R)*299+int(c.G)*587+int(c.B)*114)/1000 > prestigeInk {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if ink.Dx() < prestigeSide || ink.Dy() < prestigeSide {
		return nil
	}
	if aspect := float64(ink.Dx()) / float64(ink.Dy()); aspect < minPrestigeAspect || aspect > maxPrestigeAspect {
		return nil
	}
	out := image.NewRGBA(image.Rect(0, 0, prestigeSide, prestigeSide))
	xdraw.ApproxBiLinear.Scale(out, out.Bounds(), flat, ink, draw.Src, nil)
	return out
}

// prestigeDiff is the mean difference of a colour between two squares.
func prestigeDiff(a, b *image.RGBA) float64 {
	sum := 0
	for i := range a.Pix {
		if i%4 == 3 {
			continue
		}
		d := int(a.Pix[i]) - int(b.Pix[i])
		sum += max(d, -d)
	}
	return float64(sum) / float64(prestigeSide*prestigeSide*3)
}
