package ocr

import (
	"os"
	"path/filepath"
)

// languages are the game languages the OCR engines read, by MAYAK'
// language code (see internal/locale): Windows OCR's language tag, and
// Tesseract's model with the MAYAK model fine-tuned on Escape from Tarkov
// titles (tools/ocrtrain), which replaces it when the tessdata directory has
// it. Adding a language here, with its models bundled, lets MAYAK read it.
var languages = map[string]struct{ windows, tesseract, model string }{
	"en": {"en-US", "eng", "eft"},
	"ja": {"ja-JP", "jpn", "eftjpn"},
}

func windowsLanguage(language string) string {
	if l, ok := languages[language]; ok {
		return l.windows
	}
	return "auto"
}

// tesseractModel is Tesseract's model for language: the MAYAK model when
// dataDir has it, else Tesseract's own.
func tesseractModel(language, dataDir string) string {
	l := languages[language]
	if dataDir != "" {
		if _, err := os.Stat(filepath.Join(dataDir, l.model+".traineddata")); err == nil {
			return l.model
		}
	}
	return l.tesseract
}

// tesseractLanguage is Tesseract's -l for a game in language. Other languages
// read with English too: titles mix in English words, digits and names.
func tesseractLanguage(language, dataDir string) string {
	english := tesseractModel("en", dataDir)
	if _, ok := languages[language]; ok && language != "en" {
		return tesseractModel(language, dataDir) + "+" + english
	}
	return english
}
