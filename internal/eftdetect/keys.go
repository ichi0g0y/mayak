package eftdetect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// gameSettings is EFT's settings folder (%AppData%\Battlestate Games\Escape
// from Tarkov\Settings, wherever Windows has %AppData%).
func gameSettings(name string) string {
	config, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(config, "Battlestate Games", "Escape from Tarkov", "Settings", name)
}

// ScreenshotKeys reads EFT's own screenshot key (Control.ini, MakeScreenshot):
// each binding as a readable combination, modifiers first ("Ctrl + ;").
// known is false when the settings cannot be read (EFT not installed or never
// started); known with no keys is a screenshot key left unbound.
func ScreenshotKeys() (keys []string, known bool) {
	path := gameSettings("Control.ini")
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return screenshotKeys(data)
}

func screenshotKeys(data []byte) ([]string, bool) {
	var control struct {
		KeyBindings []struct {
			KeyName  string `json:"keyName"`
			Variants []struct {
				KeyCode []string `json:"keyCode"`
			} `json:"variants"`
		} `json:"keyBindings"`
	}
	if json.Unmarshal(data, &control) != nil {
		return nil, false
	}
	for _, binding := range control.KeyBindings {
		if binding.KeyName != "MakeScreenshot" {
			continue
		}
		keys := []string{}
		for _, variant := range binding.Variants {
			if combo := keyCombination(variant.KeyCode); combo != "" {
				keys = append(keys, combo)
			}
		}
		return keys, true
	}
	return nil, false
}

// modifierNames are Unity's modifier keys, in the order they are written.
var modifierNames = []struct{ code, name string }{
	{"LeftControl", "Ctrl"}, {"RightControl", "Ctrl"},
	{"LeftShift", "Shift"}, {"RightShift", "Shift"},
	{"LeftAlt", "Alt"}, {"RightAlt", "Alt"},
}

// keyNames are Unity key codes written as the key shows them.
var keyNames = map[string]string{
	"Semicolon": ";", "Quote": "'", "Comma": ",", "Period": ".", "Slash": "/", "Backslash": `\`,
	"LeftBracket": "[", "RightBracket": "]", "Minus": "-", "Equals": "=", "BackQuote": "`",
	"Print": "PrintScreen", "SysReq": "PrintScreen", "Return": "Enter", "KeypadEnter": "Num Enter",
	"PageUp": "PageUp", "PageDown": "PageDown", "Insert": "Insert", "Delete": "Delete", "Home": "Home", "End": "End",
}

// keyCombination writes one binding's key codes: modifiers first, then the
// key ("Ctrl + ;"); "" for none.
func keyCombination(codes []string) string {
	var mods, rest []string
	for _, code := range codes {
		if code == "" || code == "None" {
			continue
		}
		modifier := ""
		for _, m := range modifierNames {
			if m.code == code {
				modifier = m.name
			}
		}
		if modifier != "" {
			if !strings.Contains(strings.Join(mods, "|"), modifier) {
				mods = append(mods, modifier)
			}
			continue
		}
		rest = append(rest, keyName(code))
	}
	return strings.Join(append(mods, rest...), " + ")
}

func keyName(code string) string {
	if name, ok := keyNames[code]; ok {
		return name
	}
	switch {
	case strings.HasPrefix(code, "Alpha") && len(code) == 6:
		return code[5:]
	case strings.HasPrefix(code, "Keypad"):
		return "Num " + strings.TrimPrefix(code, "Keypad")
	case strings.HasPrefix(code, "Mouse"):
		// Unity counts the mouse's buttons from 0 (left).
		if n, err := strconv.Atoi(strings.TrimPrefix(code, "Mouse")); err == nil {
			return "Mouse" + strconv.Itoa(n+1)
		}
	}
	return code
}
