
package app

import (
	"context"
	"image"
	"os/exec"
	"slices"
	"strings"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/eftdetect"
	"github.com/local/mayak/internal/locale"
	"github.com/local/mayak/internal/ocr"
)

func tesseractUnavailable() bool {
	if _, ok := ocr.BundledTesseract(); ok {
		return false
	}
	_, err := exec.LookPath("tesseract")
	return err != nil
}

// ocrLanguage resolves "auto" to the game's own display language, so English
// titles are read as English even on a Japanese Windows (whose OCR would
// otherwise misread them), falling back to the Windows language settings.
func ocrLanguage(settings config.Settings) string {
	if settings.GameLanguage == "en" || slices.Contains(locale.Languages, settings.GameLanguage) {
		return settings.GameLanguage
	}
	if language := eftdetect.GameLanguage(); language != "" {
		return language
	}
	return "auto"
}

// tesseractEngine uses the configured tesseract.exe, else the one bundled
// with MAYAK, else Tesseract on PATH.
func tesseractEngine(settings config.Settings) ocr.Tesseract {
	engine := ocr.Tesseract{Executable: settings.TesseractPath}
	if engine.Executable == "" {
		if bundled, ok := ocr.BundledTesseract(); ok {
			engine = bundled
		}
	}
	engine.Language = ocrLanguage(settings)
	return engine
}

func recognizeQuestTitle(ctx context.Context, settings config.Settings, crop image.Image) (string, error) {
	if settings.OCREngine == "tesseract" {
		raw, err := tesseractEngine(settings).Recognize(ctx, crop)
		if err == nil || settings.TesseractPath != "" {
			return raw, err
		}
	}
	return (ocr.Windows{Language: ocrLanguage(settings)}).Recognize(ctx, crop)
}

// recognizeTitleCandidates reads a title; byWindows reports that Windows OCR
// read it (chosen, or instead of a failed bundled Tesseract).
func recognizeTitleCandidates(ctx context.Context, settings config.Settings, crop image.Image) (readings []string, byWindows bool, err error) {
	if settings.OCREngine == "tesseract" {
		raw, err := tesseractEngine(settings).Recognize(ctx, crop)
		if err == nil {
			return []string{raw}, false, nil
		}
		if settings.TesseractPath != "" {
			return nil, false, err
		}
	}
	readings, err = (ocr.Windows{Language: ocrLanguage(settings)}).RecognizeCandidates(ctx, crop)
	return readings, true, err
}

// bestItemReading is the reading whose best match is strongest. Without any
// match it is the first reading, so what was read stays on record.
func firstReading(readings []string) string {
	for _, raw := range readings {
		if strings.TrimSpace(raw) != "" {
			return raw
		}
	}
	return ""
}
