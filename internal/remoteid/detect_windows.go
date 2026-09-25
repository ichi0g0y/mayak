//go:build windows

package remoteid

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/syndtr/goleveldb/leveldb"
)

var remoteIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{4}$`)

// Detect reads a copy of Chromium Local Storage. The browser profile itself is
// never modified or unlocked. tarkov.dev stores the stable code as sessionId.
func Detect() (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return "", errors.New("LOCALAPPDATA is unavailable")
	}
	roots := []string{
		filepath.Join(local, "Google", "Chrome", "User Data"),
		filepath.Join(local, "Microsoft", "Edge", "User Data"),
		filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data"),
	}
	var databases []string
	for _, root := range roots {
		entries, _ := os.ReadDir(root)
		for _, entry := range entries {
			if !entry.IsDir() || (entry.Name() != "Default" && !strings.HasPrefix(entry.Name(), "Profile ")) {
				continue
			}
			db := filepath.Join(root, entry.Name(), "Local Storage", "leveldb")
			if info, err := os.Stat(db); err == nil && info.IsDir() {
				databases = append(databases, db)
			}
		}
	}
	sort.Strings(databases)
	for _, source := range databases {
		if id := detectDatabase(source); id != "" {
			return strings.ToUpper(id), nil
		}
	}
	return "", errors.New("tarkov.dev Remote ID was not found in Chrome, Edge, or Brave")
}

func detectDatabase(source string) string {
	temp, err := os.MkdirTemp("", "mayak-remoteid-")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(temp)
	entries, err := os.ReadDir(source)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "LOCK" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(source, entry.Name()))
		if readErr == nil {
			_ = os.WriteFile(filepath.Join(temp, entry.Name()), data, 0600)
		}
	}
	db, err := leveldb.OpenFile(temp, nil)
	if err != nil {
		return ""
	}
	defer db.Close()
	iterator := db.NewIterator(nil, nil)
	defer iterator.Release()
	for iterator.Next() {
		key := strings.ToLower(string(iterator.Key()))
		if strings.Contains(key, "tarkov.dev") && strings.Contains(key, "sessionid") {
			if id := decodeValue(iterator.Value()); id != "" {
				return id
			}
		}
	}
	return ""
}

func decodeValue(value []byte) string {
	variants := []string{strings.Trim(string(value), "\x00\x01\" ")}
	for offset := 0; offset < 2; offset++ {
		data := value
		if offset == 1 && len(data) > 0 {
			data = data[1:]
		}
		if len(data) < 2 || len(data)%2 != 0 {
			continue
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(data[i*2:])
		}
		variants = append(variants, strings.Trim(string(utf16.Decode(units)), "\x00\x01\" "))
	}
	for _, candidate := range variants {
		if remoteIDPattern.MatchString(candidate) {
			return candidate
		}
	}
	return ""
}
