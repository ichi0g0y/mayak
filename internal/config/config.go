package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/userdata"
)

var saveMu sync.Mutex

type Settings struct {
	GameLanguage string `json:"gameLanguage"`
	QuestSite    string `json:"questSite"`
	// QuestSites is the task sites in the order they are tried: a task opens
	// on the first that has its page. QuestSite is the first, for the
	// versions that know one site only.
	QuestSites []string `json:"questSites"`
	// ToastsOff are the kinds of notices over the pages turned off in the
	// settings (app.toastCategories: tracker, sound, recognition, squad).
	ToastsOff           []string       `json:"toastsOff"`
	Language            string         `json:"language"`
	ScreenshotDirectory string         `json:"screenshotDirectory"`
	LogsDirectory       string         `json:"logsDirectory"`
	RemoteID            string         `json:"remoteId"`
	RemoteTargets       []RemoteTarget `json:"remoteTargets"`
	BrowserRemoteID     string         `json:"browserRemoteId"`
	Map                 string         `json:"map"`
	GameMode            string         `json:"gameMode"`
	OCREngine           string         `json:"ocrEngine"`
	TesseractPath       string         `json:"tesseractPath"`
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
	// ErrorSound is a screenshot that could not be read; it once also
	// covered a task matching nothing and a failed tarkov.dev connection,
	// which have their own now (settings saved before take its choice).
	ErrorSoundEnabled          bool   `json:"errorSoundEnabled"`
	ErrorSoundPath             string `json:"errorSoundPath"`
	TaskNotMatchedSoundEnabled bool   `json:"taskNotMatchedSoundEnabled"`
	TaskNotMatchedSoundPath    string `json:"taskNotMatchedSoundPath"`
	RemoteErrorSoundEnabled    bool   `json:"remoteErrorSoundEnabled"`
	RemoteErrorSoundPath       string `json:"remoteErrorSoundPath"`
	ItemSoundEnabled           bool   `json:"itemSoundEnabled"`
	ItemSoundPath              string `json:"itemSoundPath"`
	ItemNotMatchedSoundEnabled bool   `json:"itemNotMatchedSoundEnabled"`
	ItemNotMatchedSoundPath    string `json:"itemNotMatchedSoundPath"`
	SoundVolume                int    `json:"soundVolume"`
	// SoundVoice is the built-in voice the notifications speak with (a
	// pack of internal/sound/voices), "beep" for the built-in beeps, or
	// empty for the language's default (app_sound.go baseVoice).
	SoundVoice string `json:"soundVoice"`
	// SoundVoices is the voice a notification speaks with instead (its kind,
	// sound.Kind, to a pack or "beep"); one not in it takes SoundVoice.
	SoundVoices map[string]string `json:"soundVoices"`
	// SoundVolumeOffsets turns a notification (its kind) up or down from
	// SoundVolume, in points (-50 to +50).
	SoundVolumeOffsets map[string]int `json:"soundVolumeOffsets"`
	// SoundDelays is how many seconds a notification (its kind) waits before
	// it plays (0 to SoundDelayMax), so it does not speak over the game; a
	// kind not in it has its default (soundDelayDefaults), 0 for most.
	SoundDelays          map[string]int `json:"soundDelays"`
	AutoStartMonitoring  bool           `json:"autoStartMonitoring"`
	OpenMapOnRaidStart   bool           `json:"openMapOnRaidStart"`
	NavigateMapOnShot    bool           `json:"navigateMapOnPositionScreenshot"`
	TarkovTrackerEnabled bool           `json:"tarkovTrackerEnabled"`
	MatchFoundSound      bool           `json:"matchFoundSoundEnabled"`
	MatchFoundSoundPath  string         `json:"matchFoundSoundPath"`
	RaidStartSound       bool           `json:"raidStartSoundEnabled"`
	RaidStartSoundPath   string         `json:"raidStartSoundPath"`
	RunThroughSound      bool           `json:"runThroughSoundEnabled"`
	RunThroughSoundPath  string         `json:"runThroughSoundPath"`
	RunThroughSeconds    int            `json:"runThroughSeconds"`
	GameExitSound        bool           `json:"gameExitSoundEnabled"`
	GameExitSoundPath    string         `json:"gameExitSoundPath"`
	GameStartSound       bool           `json:"gameStartSoundEnabled"`
	GameStartSoundPath   string         `json:"gameStartSoundPath"`
	QuestItemsSound      bool           `json:"questItemsSoundEnabled"`
	QuestItemsSoundPath  string         `json:"questItemsSoundPath"`
	TaskFailedSound      bool           `json:"taskFailedSoundEnabled"`
	TaskFailedSoundPath  string         `json:"taskFailedSoundPath"`
	StartMinimized       bool           `json:"startMinimized"`
	// MinimizeToTray removes the taskbar entry when the window is minimized;
	// CloseToTray keeps the app running in the tray when it is closed.
	MinimizeToTray bool `json:"minimizeToTray"`
	CloseToTray    bool `json:"closeToTray"`
	// KeepPriority puts MAYAK and its window's WebView2 processes back to
	// normal priority when a priority manager (Process Lasso) lowered them
	// (priority_windows.go); off unless such a tool makes the window sluggish.
	KeepPriority    bool `json:"keepPriority"`
	LaunchAtStartup bool `json:"launchAtStartup"`
	// AutoUpdate checks GitHub Releases for a newer MAYAK, downloads it in
	// the background and installs it when MAYAK quits.
	AutoUpdate bool `json:"autoUpdate"`
	// UpdateChannel is where updates come from: "stable" (tagged releases)
	// or "nightly" (also the nightly build, whichever is newer).
	UpdateChannel    string `json:"updateChannel"`
	WindowX          int    `json:"windowX"`
	WindowY          int    `json:"windowY"`
	WindowWidth      int    `json:"windowWidth"`
	WindowHeight     int    `json:"windowHeight"`
	WindowConfigured bool   `json:"windowConfigured"`
}

// SoundDelayMax is the longest a notification waits, in seconds.
const SoundDelayMax = 15

// soundDelayDefaults are the delays of the notifications that have one
// without being set: back from a raid and a task failed come as the game
// makes its own sounds (frontend/src/settings-model.ts SOUND_DELAY_DEFAULTS).
var soundDelayDefaults = map[string]int{"questItems": 3, "taskFailed": 3}

// SoundDelay is how many seconds the notification of kind waits: its own
// delay, else its default.
func (s Settings) SoundDelay(kind string) int {
	if seconds, ok := s.SoundDelays[kind]; ok {
		return min(max(seconds, 0), SoundDelayMax)
	}
	return soundDelayDefaults[kind]
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
	return Settings{Language: "ja", GameMode: "auto", OCREngine: "tesseract", ScreenshotRetainCount: 500, ScreenshotRetainHours: 168, SoundsEnabled: false, QuestSoundEnabled: true, ErrorSoundEnabled: true, TaskNotMatchedSoundEnabled: true, RemoteErrorSoundEnabled: true, SoundVolume: 28, AutoStartMonitoring: true, OpenMapOnRaidStart: true, NavigateMapOnShot: true, RunThroughSeconds: 430, MatchFoundSound: true, RaidStartSound: true, RunThroughSound: true, AutoUpdate: true}
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

// Settings are kept in two files: settings.json has what belongs to this PC
// (deviceKeys: folders, window places, pairing ids, OCR, startup), and
// preferences.json the rest, key by key with when each changed, to follow
// the user to another PC (internal/userdata). Settings saved before the
// split are all in settings.json; the next save moves the preferences out.
// The sound files are preferences: they are taken into the data folder
// (internal/app/app_sound_files.go) and named from there.
var deviceKeys = map[string]bool{
	"screenshotDirectory": true, "logsDirectory": true, "tesseractPath": true,
	"ocrEngine": true, "ocrDefaultRevision": true,
	"remoteId": true, "remoteTargets": true, "browserRemoteId": true, "map": true,
	"debug": true, "saveRecognitionDebug": true, "keepPriority": true, "launchAtStartup": true,
	"windowX": true, "windowY": true, "windowWidth": true, "windowHeight": true, "windowConfigured": true,
}

// IsDeviceKey tells a setting (its JSON name) that belongs to this PC.
func IsDeviceKey(key string) bool { return deviceKeys[key] }

func preferencesPath(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), "preferences.json")
}

func loadFile(p string) (Settings, error) {
	fields := map[string]json.RawMessage{}
	b, err := os.ReadFile(p)
	var readErr error
	switch {
	case err == nil:
		if json.Unmarshal(b, &fields) != nil {
			readErr = errors.New("settings.json is damaged")
			fields = map[string]json.RawMessage{}
			if backup, backupErr := os.ReadFile(p + ".bak"); backupErr == nil && json.Unmarshal(backup, &fields) == nil {
				readErr = nil
			}
		}
	case !os.IsNotExist(err):
		readErr = err
	}
	prefs, _, prefsErr := userdata.LoadKeyed(preferencesPath(p))
	for key, value := range prefs.Values {
		if !deviceKeys[key] {
			fields[key] = value
		}
	}
	merged, err := json.Marshal(fields)
	if err != nil {
		return defaults(), err
	}
	settings, err := decodeSettings(merged)
	if err != nil {
		return defaults(), err
	}
	if readErr == nil {
		readErr = prefsErr
	}
	return settings, readErr
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
		if _, ok := fields["taskNotMatchedSoundEnabled"]; !ok {
			s.TaskNotMatchedSoundEnabled, s.TaskNotMatchedSoundPath = s.ErrorSoundEnabled, s.ErrorSoundPath
		}
		if _, ok := fields["remoteErrorSoundEnabled"]; !ok {
			s.RemoteErrorSoundEnabled, s.RemoteErrorSoundPath = s.ErrorSoundEnabled, s.ErrorSoundPath
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
	return saveFile(p, s, time.Now())
}

// saveFile writes this PC's settings to settings.json and the preferences
// that changed (stamped with now) to preferences.json.
func saveFile(p string, s Settings, now time.Time) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(b, &fields); err != nil {
		return err
	}
	device := map[string]json.RawMessage{}
	preferences := map[string]json.RawMessage{}
	for key, value := range fields {
		if deviceKeys[key] {
			device[key] = value
		} else {
			preferences[key] = value
		}
	}
	prefsFile := preferencesPath(p)
	prefs, existed, _ := userdata.LoadKeyed(prefsFile)
	changed := prefs.Set(preferences, now)
	// A setting MAYAK no longer has goes (the hideout error alert did).
	for key := range prefs.Values {
		if _, ok := preferences[key]; !ok {
			delete(prefs.Values, key)
			delete(prefs.UpdatedAt, key)
			changed = true
		}
	}
	if changed || !existed {
		if err = userdata.SaveKeyed(prefsFile, prefs); err != nil {
			return err
		}
	}
	out, err := json.MarshalIndent(device, "", "  ")
	if err != nil {
		return err
	}
	return userdata.WriteWithBackup(p, out)
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
