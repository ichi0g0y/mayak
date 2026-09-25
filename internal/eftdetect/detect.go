package eftdetect

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Result struct {
	ScreenshotDirectory string `json:"screenshotDirectory"`
	LogsDirectory       string `json:"logsDirectory"`
}

type launcherSettings struct {
	GamesRootDir string `json:"gamesRootDir"`
}

// Detect reads ordinary EFT folders and the launcher's public installation setting only.
func Detect() (Result, error) {
	var result Result
	home, _ := os.UserHomeDir()
	result.ScreenshotDirectory = firstDirectory([]string{
		filepath.Join(home, "Documents", "Escape from Tarkov", "Screenshots"),
		filepath.Join(home, "OneDrive", "Documents", "Escape from Tarkov", "Screenshots"),
	})

	roots := launcherRoots(home)
	for drive := 'C'; drive <= 'Z'; drive++ {
		root := string(drive) + `:\`
		roots = append(roots, filepath.Join(root, "Battlestate Games"), filepath.Join(root, "Games"))
	}
	for _, root := range unique(roots) {
		for _, gameName := range []string{"Escape from Tarkov", "EFT", "EFT (Live)"} {
			gameDir := filepath.Join(root, gameName)
			if result.LogsDirectory == "" && isDirectory(filepath.Join(gameDir, "Logs")) {
				result.LogsDirectory = filepath.Join(gameDir, "Logs")
			}
			if result.ScreenshotDirectory == "" && isDirectory(filepath.Join(gameDir, "Screenshots")) {
				result.ScreenshotDirectory = filepath.Join(gameDir, "Screenshots")
			}
		}
	}
	if result.ScreenshotDirectory == "" && result.LogsDirectory == "" {
		return result, errors.New("EFTのScreenshots/Logsフォルダを自動検出できませんでした")
	}
	return result, nil
}

func launcherRoots(home string) []string {
	settingsPath := filepath.Join(home, "AppData", "Roaming", "Battlestate Games", "BsgLauncher", "settings")
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil
	}
	var settings launcherSettings
	if json.Unmarshal(b, &settings) != nil || settings.GamesRootDir == "" {
		return nil
	}
	return []string{filepath.Clean(settings.GamesRootDir)}
}

func firstDirectory(paths []string) string {
	for _, path := range paths {
		if isDirectory(path) {
			return filepath.Clean(path)
		}
	}
	return ""
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = filepath.Clean(value)
		if value == "." {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// GameLanguage returns the game's display language from its settings
// (%AppData%\Battlestate Games\Escape from Tarkov\Settings\Game.ini) as "ja"
// or "en", or "" when it is unknown. Other Latin-script languages read as "en".
func GameLanguage() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, "AppData", "Roaming", "Battlestate Games", "Escape from Tarkov", "Settings", "Game.ini"))
	if err != nil {
		return ""
	}
	return gameLanguage(data)
}

func gameLanguage(data []byte) string {
	var settings struct {
		Language string `json:"Language"`
	}
	if json.Unmarshal(data, &settings) != nil {
		return ""
	}
	switch strings.ToLower(settings.Language) {
	case "jp", "ja":
		return "ja"
	case "en", "es", "es-mx", "fr", "ge", "de", "it", "pl", "po", "pt", "cz", "hu", "sk", "tu", "tr":
		return "en"
	}
	return ""
}
