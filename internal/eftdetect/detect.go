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

// Detect finds EFT's folders from where the game puts them: its screenshots
// under Windows' own Documents folder (EFT and its logs say no other place),
// its logs in the game's folder, found from the install location Windows
// has, the launcher's setting, or the usual folders of each drive.
func Detect() (Result, error) {
	var result Result
	home, _ := os.UserHomeDir()
	result.ScreenshotDirectory = firstDirectory(screenshotPlaces(documentsDirectory(), home))

	if game := installedGame(); game != "" && isDirectory(filepath.Join(game, "Logs")) {
		result.LogsDirectory = filepath.Join(filepath.Clean(game), "Logs")
	}
	roots := launcherRoots()
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
		}
	}
	if result.ScreenshotDirectory == "" && result.LogsDirectory == "" {
		return result, errors.New("EFTのScreenshots/Logsフォルダを自動検出できませんでした")
	}
	return result, nil
}

// screenshotPlaces are where EFT's screenshots may be: under Windows'
// Documents folder first, then the home's Documents and OneDrive's (in case
// Windows does not answer).
func screenshotPlaces(documents, home string) []string {
	var places []string
	if documents != "" {
		places = append(places, filepath.Join(documents, "Escape from Tarkov", "Screenshots"))
	}
	if home != "" {
		places = append(places,
			filepath.Join(home, "Documents", "Escape from Tarkov", "Screenshots"),
			filepath.Join(home, "OneDrive", "Documents", "Escape from Tarkov", "Screenshots"))
	}
	return places
}

// launcherRoots are the games folder in the launcher's settings (under
// %AppData%, wherever Windows has it).
func launcherRoots() []string {
	config, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	settingsPath := filepath.Join(config, "Battlestate Games", "BsgLauncher", "settings")
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
	path := gameSettings("Game.ini")
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
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
