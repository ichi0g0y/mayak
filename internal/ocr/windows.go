package ocr

import (
	"context"
	_ "embed"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"runtime"
	"strings"
)

//go:embed windows_ocr.ps1
var windowsOCRScript []byte

type Windows struct{ Language string }

// RecognizeCandidates reads both the original crop and thresholded variants.
// Quest matching can then select the reading that best fits the known catalog.
func (w Windows) RecognizeCandidates(ctx context.Context, img image.Image) ([]string, error) {
	readings := make([]string, 0, 4)
	raw, err := w.Recognize(ctx, img)
	if err == nil && strings.TrimSpace(raw) != "" {
		readings = append(readings, strings.TrimSpace(raw))
	}
	alternatives, alternativeErr := w.RecognizeAlternatives(ctx, img)
	readings = append(readings, alternatives...)
	if len(readings) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, alternativeErr
	}
	return uniqueReadings(readings), nil
}

// RecognizeAlternatives is intended as a second pass only when the original
// OCR reading does not confidently match a known quest.
func (w Windows) RecognizeAlternatives(ctx context.Context, img image.Image) ([]string, error) {
	variants := []image.Image{thresholdImage(img, 110), thresholdImage(img, 140), thresholdImage(img, 170)}
	readings := make([]string, 0, len(variants))
	seen := make(map[string]struct{}, len(variants))
	var lastErr error
	for _, variant := range variants {
		raw, err := w.Recognize(ctx, variant)
		if err != nil {
			lastErr = err
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		key := strings.ToLower(raw)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		readings = append(readings, raw)
	}
	if len(readings) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return readings, nil
}

func uniqueReadings(readings []string) []string {
	unique := make([]string, 0, len(readings))
	seen := make(map[string]struct{}, len(readings))
	for _, raw := range readings {
		key := strings.ToLower(strings.TrimSpace(raw))
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, strings.TrimSpace(raw))
	}
	return unique
}

func (w Windows) Recognize(ctx context.Context, img image.Image) (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("Windows OCR is available only on Windows")
	}
	imageFile, err := os.CreateTemp("", "mayak-windows-ocr-*.png")
	if err != nil {
		return "", err
	}
	imagePath := imageFile.Name()
	defer os.Remove(imagePath)
	if err = png.Encode(imageFile, upscale2x(img)); err != nil {
		_ = imageFile.Close()
		return "", err
	}
	if err = imageFile.Close(); err != nil {
		return "", err
	}

	return sharedWindowsWorker.recognize(ctx, imagePath, windowsLanguage(w.Language))
}

// upscale2x doubles the crop and surrounds it with a margin of its corner
// color: Windows OCR returns nothing for text that touches the image edge.
func upscale2x(src image.Image) image.Image {
	const margin = 16
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*2+2*margin, b.Dy()*2+2*margin))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(src.At(b.Min.X, b.Min.Y)), image.Point{}, draw.Src)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			pixel := src.At(b.Min.X+x, b.Min.Y+y)
			dst.Set(margin+2*x, margin+2*y, pixel)
			dst.Set(margin+2*x+1, margin+2*y, pixel)
			dst.Set(margin+2*x, margin+2*y+1, pixel)
			dst.Set(margin+2*x+1, margin+2*y+1, pixel)
		}
	}
	return dst
}

func thresholdImage(src image.Image, threshold uint8) image.Image {
	b := src.Bounds()
	dst := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			gray := color.GrayModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.Gray).Y
			if gray < threshold {
				gray = 0
			} else {
				gray = 255
			}
			dst.SetGray(x, y, color.Gray{Y: gray})
		}
	}
	return dst
}
