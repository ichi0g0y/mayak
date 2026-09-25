package config

import (
	"encoding/json"
	"github.com/local/mayak/internal/appdir"
	"os"
	"path/filepath"
	"sync"
)

var saveMu sync.Mutex

type Settings struct {
	GameLanguage              string         `json:"gameLanguage"`
	QuestSite                 string         `json:"questSite"`
	HideoutErrorNotifications bool           `json:"hideoutErrorNotifications"`
	HideoutErrorSoundPath     string         `json:"hideoutErrorSoundPath"`
	Language                  string         `json:"language"`
	ScreenshotDirectory       string         `json:"screenshotDirectory"`
	LogsDirectory             string         `json:"logsDirectory"`
	RemoteID                  string         `json:"remoteId"`
	RemoteTargets             []RemoteTarget `json:"remoteTargets"`
	BrowserRemoteID           string         `json:"browserRemoteId"`
	Map                       string         `json:"map"`
	GameMode                  string         `json:"gameMode"`
	OCREngine                 string         `json:"ocrEngine"`
	TesseractPath             string         `json:"tesseractPath"`
	// OCRDefaultRevision records one-time moves to a new default OCR engine.
	OCRDefaultRevision    int    `json:"ocrDefaultRevision"`
	Debug                 bool   `json:"debug"`
	SaveRecognitionDebug  bool   `json:"saveRecognitionDebug"`
	ScreenshotCleanup     bool   `json:"screenshotCleanup"`
	ScreenshotRetainCount int    `json:"screenshotRetainCount"`
	ScreenshotRetainHours int    `json:"screenshotRetainHours"`
	SoundsEnabled         bool   `json:"soundsEnabled"`
	QuestSoundEnabled     bool   `json:"questSoundEnabled"`
	QuestSoundPath        string `json:"questSoundPath"`
	ErrorSoundEnabled     bool   `json:"errorSoundEnabled"`
	ErrorSoundPath        string `json:"errorSoundPath"`
	SoundVolume           int    `json:"soundVolume"`
	AutoStartMonitoring   bool   `json:"autoStartMonitoring"`
	OpenMapOnRaidStart    bool   `json:"openMapOnRaidStart"`
	NavigateMapOnShot     bool   `json:"navigateMapOnPositionScreenshot"`
	TarkovTrackerEnabled  bool   `json:"tarkovTrackerEnabled"`
	MatchFoundSound       bool   `json:"matchFoundSoundEnabled"`
	MatchFoundSoundPath   string `json:"matchFoundSoundPath"`
	RaidStartSound        bool   `json:"raidStartSoundEnabled"`
	RaidStartSoundPath    string `json:"raidStartSoundPath"`
	RunThroughSound       bool   `json:"runThroughSoundEnabled"`
	RunThroughSoundPath   string `json:"runThroughSoundPath"`
	RunThroughSeconds     int    `json:"runThroughSeconds"`
	QuestItemsSound       bool   `json:"questItemsSoundEnabled"`
	QuestItemsSoundPath   string `json:"questItemsSoundPath"`
	RestartTasksSound     bool   `json:"restartTasksSoundEnabled"`
	RestartTasksSoundPath string `json:"restartTasksSoundPath"`
	StartMinimized        bool   `json:"startMinimized"`
	// MinimizeToTray removes the taskbar entry when the window is minimized;
	// CloseToTray keeps the app running in the tray when it is closed.
	MinimizeToTray   bool `json:"minimizeToTray"`
	CloseToTray      bool `json:"closeToTray"`
	LaunchAtStartup  bool `json:"launchAtStartup"`
	WindowX          int  `json:"windowX"`
	WindowY          int  `json:"windowY"`
	WindowWidth      int  `json:"windowWidth"`
	WindowHeight     int  `json:"windowHeight"`
	WindowConfigured bool `json:"windowConfigured"`
}

type RemoteTarget struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Map   bool   `json:"map"`
	Tasks bool   `json:"tasks"`
}

type WindowState struct {
	ScreenID   string `json:"screenId,omitempty"`
	ScreenName string `json:"screenName,omitempty"`
	ScreenX    int    `json:"screenX,omitempty"`
	ScreenY    int    `json:"screenY,omitempty"`
	Maximized  bool   `json:"maximized"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Configured bool   `json:"configured"`
}

func defaults() Settings {
	return Settings{Language: "ja", GameMode: "auto", OCREngine: "tesseract", ScreenshotRetainCount: 500, ScreenshotRetainHours: 168, SoundsEnabled: true, QuestSoundEnabled: true, ErrorSoundEnabled: true, SoundVolume: 28, AutoStartMonitoring: true, OpenMapOnRaidStart: true, NavigateMapOnShot: true, RunThroughSeconds: 430}
}
func path() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, appdir.Name, "settings.json"), nil
}
func Load() (Settings, error) {
	p, err := path()
	if err != nil {
		return defaults(), err
	}
	return loadFile(p)
}

func loadFile(p string) (Settings, error) {
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return defaults(), nil
	}
	if err != nil {
		return defaults(), err
	}
	settings, decodeErr := decodeSettings(b)
	if decodeErr == nil {
		return settings, nil
	}
	if backup, backupErr := os.ReadFile(p + ".bak"); backupErr == nil {
		if recovered, recoveryErr := decodeSettings(backup); recoveryErr == nil {
			return recovered, nil
		}
	}
	return defaults(), decodeErr
}

func decodeSettings(b []byte) (Settings, error) {
	s := defaults()
	if err := json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	// minimizeToTray used to cover closing too; settings saved before
	// closeToTray existed keep that behavior.
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) == nil {
		if _, ok := fields["closeToTray"]; !ok {
			s.CloseToTray = s.MinimizeToTray
		}
	}
	return s, nil
}
func Save(s Settings) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	p, err := path()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if current, readErr := os.ReadFile(p); readErr == nil && json.Valid(current) {
		_ = writeFileAtomic(p+".bak", current)
	}
	return writeFileAtomic(p, b)
}

func LoadWindow() (WindowState, error)        { return loadWindowFile("window.json") }
func SaveWindow(state WindowState) error      { return saveWindowFile("window.json", state) }
func LoadPopupWindow() (WindowState, error)   { return loadWindowFile("popup.json") }
func SavePopupWindow(state WindowState) error { return saveWindowFile("popup.json", state) }

func loadWindowFile(name string) (WindowState, error) {
	p, err := path()
	if err != nil {
		return WindowState{}, err
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(p), name))
	if os.IsNotExist(err) {
		return WindowState{}, nil
	}
	if err != nil {
		return WindowState{}, err
	}
	var state WindowState
	return state, json.Unmarshal(b, &state)
}

func saveWindowFile(name string, state WindowState) error {
	p, err := path()
	if err != nil {
		return err
	}
	windowPath := filepath.Join(filepath.Dir(p), name)
	if err = os.MkdirAll(filepath.Dir(windowPath), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(windowPath, b)
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	temporaryPath := file.Name()
	defer os.Remove(temporaryPath)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
