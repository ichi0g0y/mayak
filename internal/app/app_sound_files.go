package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/sound"
	"github.com/local/mayak/internal/userdata"
)

// A sound file chosen for a notification is copied into the data folder
// (sounds/<hash>-<name>) and the setting holds that name, not a path on this
// PC, so it follows the settings to another PC (internal/userdata). A path
// chosen before is taken in once, at the start (importSoundFiles).
const soundsDir = "sounds"

const maxSoundFileBytes = 32 << 20

var soundRef = regexp.MustCompile(`^sounds/[0-9a-f]{16}-[^/\\]{1,100}$`)

// soundFields are the settings holding a notification's sound file.
func soundFields(s *config.Settings) []*string {
	return []*string{
		&s.QuestSoundPath, &s.ErrorSoundPath, &s.TaskNotMatchedSoundPath,
		&s.RemoteErrorSoundPath, &s.ItemSoundPath, &s.ItemNotMatchedSoundPath, &s.MatchFoundSoundPath,
		&s.RaidStartSoundPath, &s.RunThroughSoundPath, &s.GameStartSoundPath, &s.QuestItemsSoundPath, &s.GameExitSoundPath, &s.TaskFailedSoundPath,
	}
}

// importSoundFile copies an audio file into sounds/ and returns its name
// there ("sounds/<hash>-<name>"); the same file is kept once.
func importSoundFile(src string) (string, error) {
	if err := sound.ValidateFile(src); err != nil {
		return "", err
	}
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSoundFileBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxSoundFileBytes {
		return "", errors.New("the sound file is too large")
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:8]) + "-" + cleanSoundName(filepath.Base(src))
	dst, err := appdir.Path(soundsDir, name)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return "", err
		}
		if err := userdata.WriteAtomic(dst, data); err != nil {
			return "", err
		}
	}
	return soundsDir + "/" + name, nil
}

// cleanSoundName keeps a file name that is safe in a path and in the
// Windows audio commands (sound.ValidateFile).
func cleanSoundName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`\/:*?"<>|`, r) {
			return '_'
		}
		return r
	}, base)
	if r := []rune(base); len(r) > 80 {
		base = string(r[:80])
	}
	if strings.TrimSpace(base) == "" {
		base = "sound"
	}
	return base + ext
}

// soundFilePath is the file a setting names: a file taken in (sounds/…)
// in the data folder, or a path chosen before (kept as it is until taken
// in); "" for none or a name that is not one.
func soundFilePath(ref string) string {
	switch {
	case ref == "":
		return ""
	case soundRef.MatchString(ref):
		p, err := appdir.Path(soundsDir, strings.TrimPrefix(ref, soundsDir+"/"))
		if err != nil {
			return ""
		}
		return p
	case filepath.IsAbs(ref):
		return ref
	}
	return ""
}

// importSoundFiles takes in the sound files chosen before by their path;
// one that cannot be read keeps its path (it plays the voice or beep until
// chosen again). It tells whether any setting changed.
func importSoundFiles(s *config.Settings) bool {
	changed := false
	for _, field := range soundFields(s) {
		if *field == "" || !filepath.IsAbs(*field) {
			continue
		}
		if ref, err := importSoundFile(*field); err == nil {
			*field = ref
			changed = true
		}
	}
	return changed
}
