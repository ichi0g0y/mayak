// Package screenscale brings screenshots to the layout the detectors
// (internal/itemdetect, internal/taskdetect) are measured in: 2560 px wide.
//
// The resolution comes from the screenshot itself, so there is nothing to
// set. EFT sizes its interface by the screen's width, so a screenshot is
// scaled to 2560 px wide, keeping its aspect ratio: 16:9 (1920×1080,
// 3840×2160, ...) becomes 2560×1440, and 16:10 (1920×1200) 2560×1600, whose
// extra height is room below the same layout. A wider screen (ultrawide,
// 21:9 and 32:9) has EFT's menus in its center 16:9 with background on both
// sides (seen at 2520×1080, 2026-10-10): that center is cut out first.
// Narrower screens (4:3, 5:4, 3:2) are not supported.
package screenscale

import (
	"errors"
	"image"

	"golang.org/x/image/draw"
)

// Width and Height are the layout the detectors are measured in: its width,
// and its height at 16:9 (the least a screenshot has once scaled).
const Width, Height = 2560, 1440

// ErrUnsupported is returned for screenshots narrower than 16:10.
var ErrUnsupported = errors.New("unsupported screenshot resolution: only 16:10 and wider screenshots are supported")

// To1440p returns img at 2560 px wide for detection: unchanged when it
// already is, else scaled without blending pixels, so thin window borders
// keep their exact colors (a smooth scale blurs them into the background).
func To1440p(img image.Image) (image.Image, error) {
	return scale(img, draw.NearestNeighbor)
}

// Smooth1440p is To1440p for reading text: scaled smoothly, so glyphs keep
// their shape for OCR.
func Smooth1440p(img image.Image) (image.Image, error) {
	return scale(img, draw.CatmullRom)
}

func scale(img image.Image, scaler draw.Scaler) (image.Image, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	// From 16:10 to 16:9, with a little room for odd sizes like 1366×768.
	if h < 360 || w*100 < h*159 {
		return nil, ErrUnsupported
	}
	// Wider than that: the center 16:9, where EFT draws its menus.
	if w*100 > h*179 {
		center := h * 16 / 9
		left := b.Min.X + (w-center)/2
		b = image.Rect(left, b.Min.Y, left+center, b.Max.Y)
		w = center
	}
	if w == Width && b == img.Bounds() {
		return img, nil
	}
	height := h * Width / w
	dst := image.NewRGBA(image.Rect(0, 0, Width, height))
	scaler.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst, nil
}
