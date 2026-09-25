package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type ProcessedScreenshot struct {
	Path             string `json:"path"`
	Size             int64  `json:"size"`
	ModifiedUnixNano int64  `json:"modifiedUnixNano"`
}

func FingerprintScreenshot(path string) (ProcessedScreenshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		return ProcessedScreenshot{}, err
	}
	return ProcessedScreenshot{
		Path:             filepath.Clean(path),
		Size:             info.Size(),
		ModifiedUnixNano: info.ModTime().UnixNano(),
	}, nil
}

func (s ProcessedScreenshot) Matches(other ProcessedScreenshot) bool {
	return s.Path != "" && other.Path != "" &&
		strings.EqualFold(filepath.Clean(s.Path), filepath.Clean(other.Path)) &&
		s.Size == other.Size && s.ModifiedUnixNano == other.ModifiedUnixNano
}

func LoadProcessedScreenshot() (ProcessedScreenshot, error) {
	statePath, err := processedScreenshotPath()
	if err != nil {
		return ProcessedScreenshot{}, err
	}
	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return ProcessedScreenshot{}, nil
	}
	if err != nil {
		return ProcessedScreenshot{}, err
	}
	var state ProcessedScreenshot
	return state, json.Unmarshal(data, &state)
}

func SaveProcessedScreenshot(state ProcessedScreenshot) error {
	statePath, err := processedScreenshotPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(statePath), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(statePath, data)
}

func processedScreenshotPath() (string, error) {
	settingsPath, err := path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(settingsPath), "processed-screenshot.json"), nil
}
