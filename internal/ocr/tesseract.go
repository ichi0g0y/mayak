package ocr

import (
	"bytes"
	"context"
	"errors"
	"github.com/local/mayak/internal/imaging"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"strings"
)

type Engine interface {
	Recognize(context.Context, image.Image) (string, error)
}

// Tesseract runs tesseract.exe. DataDir, when set, is its tessdata directory.
type Tesseract struct{ Executable, Language, DataDir string }

func (t Tesseract) Recognize(ctx context.Context, img image.Image) (string, error) {
	exe := t.Executable
	if exe == "" {
		var err error
		exe, err = exec.LookPath("tesseract")
		if err != nil {
			return "", errors.New("Tesseract is not installed or not on PATH")
		}
	}
	f, err := os.CreateTemp("", "mayak-ocr-*.png")
	if err != nil {
		return "", err
	}
	name := f.Name()
	defer os.Remove(name)
	processed := preprocess(img)
	if err = png.Encode(f, processed); err != nil {
		_ = f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	args := []string{name, "stdout", "-l", tesseractLanguage(t.Language, t.DataDir), "--psm", "7"}
	if t.DataDir != "" {
		args = append(args, "--tessdata-dir", t.DataDir)
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	configureHiddenProcess(cmd)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		return "", errors.New(strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// preprocess prepares a task or item title for Tesseract: it drops the icon
// left of the title's separator line, doubles the size, binarizes with an
// Otsu threshold (dark text becomes black on white) and trims to the text with
// a white margin. Line models are trained on such tight lines (tools/ocrtrain).
func preprocess(src image.Image) image.Image {
	b := src.Bounds()
	px := imaging.Of(src)
	luma := func(x, y int) uint8 { return px.Gray(x-b.Min.X, y-b.Min.Y) }
	left := titleStart(src, luma)
	var histogram [256]int
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := left; x < b.Max.X; x++ {
			histogram[luma(x, y)]++
		}
	}
	threshold := otsu(histogram)
	const scale, margin = 2, 12
	w, h := (b.Max.X-left)*scale, b.Dy()*scale
	dst := image.NewGray(image.Rect(0, 0, w+2*margin, h+2*margin))
	for i := range dst.Pix {
		dst.Pix[i] = 255
	}
	// Text darker than the background stays black; the reverse (light text on
	// a dark panel) is inverted so Tesseract always sees dark text.
	darkText := darkerMinority(histogram, threshold)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := luma(left+x/scale, b.Min.Y+y/scale)
			ink := v <= threshold
			if !darkText {
				ink = v > threshold
			}
			if ink {
				dst.SetGray(margin+x, margin+y, color.Gray{})
			}
		}
	}
	return trimToInk(dst, margin)
}

// trimToInk crops img to its black pixels plus margin, keeping it unchanged
// when it has none.
func trimToInk(img *image.Gray, margin int) *image.Gray {
	b := img.Bounds()
	ink := image.Rectangle{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.GrayAt(x, y).Y == 0 {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if ink.Empty() {
		return img
	}
	out := image.NewGray(image.Rect(0, 0, ink.Dx()+2*margin, ink.Dy()+2*margin))
	for i := range out.Pix {
		out.Pix[i] = 255
	}
	for y := ink.Min.Y; y < ink.Max.Y; y++ {
		copy(out.Pix[out.PixOffset(margin, margin+y-ink.Min.Y):], img.Pix[img.PixOffset(ink.Min.X, y):img.PixOffset(ink.Max.X, y)])
	}
	return out
}

// titleStart returns the first column right of a dark vertical separator in
// the left quarter of the crop (the line between a quest icon and its title),
// or the crop's left edge when there is none. Only light panels have such a
// line; on a dark panel the whole background would look like one.
func titleStart(src image.Image, luma func(x, y int) uint8) int {
	b := src.Bounds()
	var sum, count int
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			sum += int(luma(x, y))
			count++
		}
	}
	if count == 0 || sum/count < 90 {
		return b.Min.X
	}
	for x := b.Min.X + b.Dx()/4; x > b.Min.X; x-- {
		dark := 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if luma(x, y) < 60 {
				dark++
			}
		}
		if dark*10 >= b.Dy()*7 {
			return min(x+4, b.Max.X-1)
		}
	}
	return b.Min.X
}

// otsu picks the threshold that best separates the histogram in two classes.
func otsu(histogram [256]int) uint8 {
	total, sum := 0, 0
	for v, n := range histogram {
		total += n
		sum += v * n
	}
	var best uint8
	var bestVariance float64
	weightB, sumB := 0, 0
	for t := 0; t < 256; t++ {
		weightB += histogram[t]
		if weightB == 0 {
			continue
		}
		weightF := total - weightB
		if weightF == 0 {
			break
		}
		sumB += t * histogram[t]
		meanB := float64(sumB) / float64(weightB)
		meanF := float64(sum-sumB) / float64(weightF)
		if variance := float64(weightB) * float64(weightF) * (meanB - meanF) * (meanB - meanF); variance > bestVariance {
			best, bestVariance = uint8(t), variance
		}
	}
	return best
}

// darkerMinority reports whether the pixels at or below threshold are the
// minority, i.e. the text is darker than its background.
func darkerMinority(histogram [256]int, threshold uint8) bool {
	below, above := 0, 0
	for v, n := range histogram {
		if v <= int(threshold) {
			below += n
		} else {
			above += n
		}
	}
	return below <= above
}

// PrepareForTesseract applies the preprocessing Recognize uses. Training data
// goes through it too, so a trained model sees images like those it will read.
func PrepareForTesseract(img image.Image) image.Image { return preprocess(img) }
