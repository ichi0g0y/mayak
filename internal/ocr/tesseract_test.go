package ocr

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

// title draws a title crop: background, an optional dark separator line at
// x=40 with an icon left of it, and a dark or light text block at x=80..200.
func title(background, text uint8, separator bool) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, 400, 60))
	for i := range img.Pix {
		img.Pix[i] = background
	}
	for y := 0; y < 60; y++ {
		if separator {
			img.SetGray(40, y, color.Gray{Y: 20})
			img.SetGray(41, y, color.Gray{Y: 20})
		}
		for x := 80; x < 200; x++ {
			if y > 20 && y < 40 && x%6 < 3 {
				img.SetGray(x, y, color.Gray{Y: text})
			}
		}
	}
	for y := 20; y < 40; y++ {
		for x := 10; x < 30; x++ {
			img.SetGray(x, y, color.Gray{Y: 250})
		}
	}
	return img
}

func inkColumns(img image.Image) (first, last int) {
	b := img.Bounds()
	first, last = -1, -1
	for x := b.Min.X; x < b.Max.X; x++ {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y == 0 {
				if first < 0 {
					first = x
				}
				last = x
				break
			}
		}
	}
	return first, last
}

func TestPreprocessDropsTheIconOnLightPanels(t *testing.T) {
	out := preprocess(title(150, 30, true))
	// Only the text block (x=80..199, doubled) remains, with a 12px margin.
	first, last := inkColumns(out)
	if first != 12 || out.Bounds().Dx() != last+1+12 || last-first+1 > 120*2 {
		t.Errorf("ink spans %d..%d of %d; the icon or separator was kept", first, last, out.Bounds().Dx())
	}
}

func TestPreprocessKeepsDarkPanelsAndInvertsLightText(t *testing.T) {
	out := preprocess(title(25, 230, false))
	// The light icon (x=10..29) is not cut on a dark panel and becomes ink too.
	first, last := inkColumns(out)
	if first != 12 || last-first+1 != (199-10+1)*2 {
		t.Errorf("ink spans %d..%d; light text should become black and nothing be cut", first, last)
	}
}

func TestOtsuSeparatesTwoLevels(t *testing.T) {
	var h [256]int
	h[40], h[200] = 300, 700
	if th := otsu(h); th < 40 || th >= 200 {
		t.Errorf("threshold %d", th)
	}
}

func TestTesseractLanguagePrefersTheTarkovModel(t *testing.T) {
	dir := t.TempDir()
	if got := tesseractLanguage("en", dir); got != "eng" {
		t.Errorf("without the model: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "eft.traineddata"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := tesseractLanguage("en", dir); got != "eft" {
		t.Errorf("with the model: %q", got)
	}
	if got := tesseractLanguage("ja", dir); got != "jpn+eft" {
		t.Errorf("Japanese: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "eftjpn.traineddata"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := tesseractLanguage("ja", dir); got != "eftjpn+eft" {
		t.Errorf("Japanese with its model: %q", got)
	}
	if got := tesseractLanguage("en", ""); got != "eng" {
		t.Errorf("Tesseract on PATH: %q", got)
	}
}
