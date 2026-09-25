package trackerstore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/local/mayak/internal/appdir"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const CurrentVersion = 2

type Profile struct {
	AccountID string `json:"accountId"`
	ProfileID string `json:"profileId"`
	Mode      string `json:"mode"`
	FirstSeen string `json:"firstSeen"`
	LastSeen  string `json:"lastSeen"`
}

type Key struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Token     string `json:"token"`
	Mode      string `json:"mode"`
	AccountID string `json:"accountId,omitempty"`
	ProfileID string `json:"profileId,omitempty"`
	AddedAt   string `json:"addedAt"`
}

func (k Key) IsBound() bool { return k.AccountID != "" && k.ProfileID != "" }

type Document struct {
	Version  int       `json:"version"`
	Keys     []Key     `json:"keys"`
	Profiles []Profile `json:"profiles"`
}

func Empty() Document { return Document{Version: CurrentVersion, Keys: []Key{}, Profiles: []Profile{}} }

func (d Document) Clone() Document {
	return Document{Version: d.Version, Keys: append([]Key(nil), d.Keys...), Profiles: append([]Profile(nil), d.Profiles...)}
}

func (d Document) TokenFor(accountID, profileID, mode string) string {
	for _, key := range d.Keys {
		if key.Mode == mode && key.AccountID == accountID && key.ProfileID == profileID {
			return key.Token
		}
	}
	return ""
}

func (d *Document) AddKey(mode, token string, names ...string) (Key, error) {
	if !validMode(mode) || strings.TrimSpace(token) == "" {
		return Key{}, errors.New("invalid TarkovTracker key")
	}
	for _, key := range d.Keys {
		if key.Token == token {
			return Key{}, errors.New("this TarkovTracker key is already saved")
		}
	}
	id, err := randomID()
	if err != nil {
		return Key{}, err
	}
	key := Key{ID: id, Token: token, Mode: mode, AddedAt: time.Now().UTC().Format(time.RFC3339)}
	if len(names) > 0 {
		key.Name = normalizeKeyName(names[0])
	}
	d.Version = CurrentVersion
	d.Keys = append(d.Keys, key)
	return key, nil
}

func normalizeKeyName(value string) string {
	name := []rune(strings.TrimSpace(value))
	if len(name) > 80 {
		name = name[:80]
	}
	return string(name)
}

// Names are local labels. Renaming never changes credentials or profile bindings.
func (d *Document) RenameKey(id, name string) error {
	for i := range d.Keys {
		if d.Keys[i].ID == id {
			d.Keys[i].Name = normalizeKeyName(name)
			return nil
		}
	}
	return errors.New("the TarkovTracker key was not found")
}

func (d *Document) SetProfileKey(accountID, profileID, mode, keyID string) error {
	if !d.hasProfile(accountID, profileID, mode) {
		return errors.New("the EFT profile was not found in scanned logs")
	}
	for index := range d.Keys {
		key := &d.Keys[index]
		if key.AccountID == accountID && key.ProfileID == profileID && key.Mode == mode {
			key.AccountID, key.ProfileID = "", ""
		}
	}
	if keyID == "" {
		return nil
	}
	for index := range d.Keys {
		key := &d.Keys[index]
		if key.ID != keyID {
			continue
		}
		if key.Mode != mode {
			return errors.New("the key mode does not match the EFT profile")
		}
		if key.IsBound() && (key.AccountID != accountID || key.ProfileID != profileID) {
			return errors.New("the key is already assigned to another EFT profile")
		}
		key.AccountID, key.ProfileID = accountID, profileID
		return nil
	}
	return errors.New("the TarkovTracker key was not found")
}

func (d *Document) RemoveKey(id string) error {
	for index, key := range d.Keys {
		if key.ID == id {
			if key.IsBound() {
				return errors.New("unassign the key from its EFT profile before removing it")
			}
			d.Keys = append(d.Keys[:index], d.Keys[index+1:]...)
			return nil
		}
	}
	return errors.New("the TarkovTracker key was not found")
}

func (d *Document) RememberProfiles(profiles []Profile) bool {
	changed := false
	for _, incoming := range profiles {
		if incoming.AccountID == "" || incoming.ProfileID == "" || !validMode(incoming.Mode) {
			continue
		}
		found := false
		for index := range d.Profiles {
			current := &d.Profiles[index]
			if current.AccountID != incoming.AccountID || current.ProfileID != incoming.ProfileID || current.Mode != incoming.Mode {
				continue
			}
			found = true
			if current.FirstSeen == "" || (incoming.FirstSeen != "" && incoming.FirstSeen < current.FirstSeen) {
				current.FirstSeen, changed = incoming.FirstSeen, true
			}
			if incoming.LastSeen > current.LastSeen {
				current.LastSeen, changed = incoming.LastSeen, true
			}
			break
		}
		if !found {
			d.Profiles = append(d.Profiles, incoming)
			changed = true
		}
	}
	sort.Slice(d.Profiles, func(i, j int) bool { return d.Profiles[i].LastSeen > d.Profiles[j].LastSeen })
	return changed
}

func (d Document) hasProfile(accountID, profileID, mode string) bool {
	for _, profile := range d.Profiles {
		if profile.AccountID == accountID && profile.ProfileID == profileID && profile.Mode == mode {
			return true
		}
	}
	return false
}

type Store struct{ mu sync.Mutex }

func (s *Store) Load() (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := storePath()
	if err != nil {
		return Empty(), err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Empty(), nil
	}
	if err != nil {
		return Empty(), err
	}
	plain, err := unprotect(data)
	if err != nil {
		return Empty(), err
	}
	var document Document
	if err := json.Unmarshal(plain, &document); err != nil {
		return Empty(), err
	}
	if document.Version > 0 {
		document.Version = CurrentVersion
		return document, nil
	}
	var legacy struct {
		PVP      string `json:"pvp"`
		PVE      string `json:"pve"`
		Seasonal string `json:"seasonal"`
	}
	if err := json.Unmarshal(plain, &legacy); err != nil {
		return Empty(), err
	}
	document = Empty()
	for mode, token := range map[string]string{"pvp": legacy.PVP, "pve": legacy.PVE, "seasonal": legacy.Seasonal} {
		if token != "" {
			if _, err := document.AddKey(mode, token); err != nil {
				return Empty(), err
			}
		}
	}
	return document, nil
}

func (s *Store) Save(document Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	document.Version = CurrentVersion
	path, err := storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	plain, err := json.Marshal(document)
	if err != nil {
		return err
	}
	protected, err := protect(plain)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".tracker-tokens-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	_ = temporary.Chmod(0600)
	if _, err = temporary.Write(protected); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func storePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appdir.Name, "tracker-tokens.dat"), nil
}

func validMode(mode string) bool { return mode == "pvp" || mode == "pve" || mode == "seasonal" }
func randomID() (string, error) {
	value := make([]byte, 12)
	_, err := rand.Read(value)
	return hex.EncodeToString(value), err
}
