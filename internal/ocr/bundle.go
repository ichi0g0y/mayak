package ocr

import (
	"os"
	"path/filepath"
	"sync"
)

// BundledTesseract locates the Tesseract runtime shipped next to Mayak.exe
// (tesseract/tesseract.exe with its tessdata directory; see tools/tessbundle).
var BundledTesseract = sync.OnceValues(func() (Tesseract, bool) {
	exe, err := os.Executable()
	if err != nil {
		return Tesseract{}, false
	}
	dir := filepath.Join(filepath.Dir(exe), "tesseract")
	bin := filepath.Join(dir, "tesseract.exe")
	if _, err := os.Stat(bin); err != nil {
		return Tesseract{}, false
	}
	return Tesseract{Executable: bin, DataDir: filepath.Join(dir, "tessdata")}, true
})
