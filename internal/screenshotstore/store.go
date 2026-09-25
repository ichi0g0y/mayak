package screenshotstore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const DebugDirectory = "Mayak-Debug"

// legacyDebugDirectory is the folder of the app's earlier name; its data is
// still read, nothing new goes there.
const legacyDebugDirectory = "RaidLens-Debug"

type Metadata struct {
	CapturedAt       time.Time `json:"capturedAt"`
	SourceFile       string    `json:"sourceFile"`
	ScreenshotType   string    `json:"screenshotType"`
	RaidActive       bool      `json:"raidActive"`
	Map              string    `json:"map,omitempty"`
	DetectionLayout  string    `json:"detectionLayout,omitempty"`
	DetectionScore   float64   `json:"detectionScore"`
	DetectionDetails any       `json:"detectionDetails,omitempty"`
	AnalysisStage    string    `json:"analysisStage"`
	OCRRaw           string    `json:"ocrRaw,omitempty"`
	Quest            any       `json:"quest,omitempty"`
	QuestCandidates  any       `json:"questCandidates,omitempty"`
	Item             any       `json:"item,omitempty"`
	ItemCandidates   any       `json:"itemCandidates,omitempty"`
	Position         any       `json:"position,omitempty"`
	Error            string    `json:"error,omitempty"`
}

type CleanupOptions struct {
	MaxCount int
	MaxAge   time.Duration
	Protect  string
	Now      time.Time
}

func SaveDebug(root, source string, metadata Metadata, cropDataURL string) (string, error) {
	root, source, err := validateSource(root, source)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, DebugDirectory)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	stamp := metadata.CapturedAt.Local().Format("20060102-150405.000")
	label := safeLabel(metadata.ScreenshotType)
	if label == "" {
		label = "unknown"
	}
	base := fmt.Sprintf("%s_%s_%03d", stamp, label, int(metadata.DetectionScore*1000+.5))
	base = uniqueBase(dir, base)
	ext := strings.ToLower(filepath.Ext(source))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		ext = ".png"
	}
	if err = copyFile(source, filepath.Join(dir, base+"_source"+ext)); err != nil {
		return "", err
	}
	if crop, decodeErr := decodeDataURL(cropDataURL); decodeErr == nil && len(crop) > 0 {
		if err = writeAtomic(filepath.Join(dir, base+"_crop.png"), crop); err != nil {
			return "", err
		}
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", err
	}
	metaPath := filepath.Join(dir, base+".json")
	return metaPath, writeAtomic(metaPath, data)
}

func Cleanup(root string, options CleanupOptions) ([]string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "." || !filepath.IsAbs(root) || (options.MaxCount <= 0 && options.MaxAge <= 0) {
		return nil, nil
	}
	if strings.EqualFold(filepath.Clean(filepath.VolumeName(root)+string(os.PathSeparator)), root) {
		return nil, errors.New("refusing to clean a volume root")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		path     string
		modified time.Time
	}
	files := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isImage(entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, candidate{filepath.Join(root, entry.Name()), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified.After(files[j].modified) })
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	protect := filepath.Clean(options.Protect)
	deleted := make([]string, 0)
	for index, file := range files {
		overCount := options.MaxCount > 0 && index >= options.MaxCount
		overAge := options.MaxAge > 0 && now.Sub(file.modified) > options.MaxAge
		if (!overCount && !overAge) || strings.EqualFold(file.path, protect) {
			continue
		}
		if err = os.Remove(file.path); err != nil {
			return deleted, err
		}
		deleted = append(deleted, file.path)
	}
	return deleted, nil
}

func validateSource(root, source string) (string, string, error) {
	root, source = filepath.Clean(strings.TrimSpace(root)), filepath.Clean(strings.TrimSpace(source))
	if !filepath.IsAbs(root) || !filepath.IsAbs(source) || !strings.EqualFold(filepath.Dir(source), root) {
		return "", "", errors.New("debug source must be a file directly inside the screenshot directory")
	}
	return root, source, nil
}

func isImage(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg":
		return true
	}
	return false
}

func safeLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func uniqueBase(dir, base string) string {
	for n := 0; ; n++ {
		candidate := base
		if n > 0 {
			candidate = fmt.Sprintf("%s-%d", base, n)
		}
		if _, err := os.Stat(filepath.Join(dir, candidate+".json")); os.IsNotExist(err) {
			return candidate
		}
	}
}

func decodeDataURL(value string) ([]byte, error) {
	const marker = ";base64,"
	index := strings.Index(value, marker)
	if index < 0 {
		return nil, errors.New("not a base64 data URL")
	}
	return base64.StdEncoding.DecodeString(value[index+len(marker):])
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(destination), ".debug-*")
	if err != nil {
		return err
	}
	temporary := out.Name()
	defer os.Remove(temporary)
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary, destination)
}

func writeAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".debug-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
