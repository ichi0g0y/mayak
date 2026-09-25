package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// oldSuffix marks a replaced file until the next start removes it. A
// running program's file cannot be deleted or overwritten on Windows, but
// it can be renamed.
const oldSuffix = ".mayak-old"

// manifestName is the file in the staging directory that names the update
// waiting there.
const manifestName = "staged.json"

// Staged is an update that is downloaded, verified and unpacked, ready to
// be applied.
type Staged struct {
	Version string `json:"version"`
	Tag     string `json:"tag"`
	Archive string `json:"archive"`
	URL     string `json:"url"`
	// Dir holds the unpacked files, laid out as they go beside the program.
	Dir string `json:"dir"`
}

// Stage downloads the release's archive for this OS and CPU into
// stagingDir, verifies it and unpacks it. Anything staged before is
// replaced.
func (c *Client) Stage(ctx context.Context, release Release, stagingDir string, progress Progress) (Staged, error) {
	asset, ok := release.Archive()
	if !ok {
		return Staged{}, fmt.Errorf("release %s has no build for %s/%s", release.Tag, runtime.GOOS, runtime.GOARCH)
	}
	sums, err := c.Checksums(ctx, release)
	if err != nil {
		return Staged{}, err
	}
	if err := Discard(stagingDir); err != nil {
		return Staged{}, err
	}
	archive, err := c.Download(ctx, asset, sums[asset.Name], stagingDir, progress)
	if err != nil {
		return Staged{}, err
	}
	files := filepath.Join(stagingDir, "files")
	if err := Extract(archive, files); err != nil {
		return Staged{}, err
	}
	// The archive is not needed once it is unpacked.
	os.Remove(archive)
	staged := Staged{Version: release.Version(), Tag: release.Tag, Archive: asset.Name, URL: release.URL, Dir: files}
	data, err := json.MarshalIndent(staged, "", "  ")
	if err != nil {
		return Staged{}, err
	}
	if err := os.WriteFile(filepath.Join(stagingDir, manifestName), data, 0o644); err != nil {
		return Staged{}, err
	}
	return staged, nil
}

// LoadStaged returns the update waiting in stagingDir, if its files are
// still there.
func LoadStaged(stagingDir string) (Staged, bool) {
	data, err := os.ReadFile(filepath.Join(stagingDir, manifestName))
	if err != nil {
		return Staged{}, false
	}
	var staged Staged
	if json.Unmarshal(data, &staged) != nil || staged.Version == "" || staged.Dir == "" {
		return Staged{}, false
	}
	if info, err := os.Stat(staged.Dir); err != nil || !info.IsDir() {
		return Staged{}, false
	}
	return staged, true
}

// Discard removes whatever is staged.
func Discard(stagingDir string) error {
	err := os.RemoveAll(stagingDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// InstallDir is the directory the running program's files live in.
func InstallDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe), nil
}

// Apply puts the staged files in place of the program's files in
// installDir. A file that exists is renamed with oldSuffix first, so the
// running program keeps its file; CleanupOld removes those on the next
// start. When a file cannot be put in place, the files replaced so far are
// restored.
func Apply(staged Staged, installDir string) (err error) {
	var replaced [][2]string
	defer func() {
		if err == nil {
			return
		}
		for i := len(replaced) - 1; i >= 0; i-- {
			target, old := replaced[i][0], replaced[i][1]
			os.Remove(target)
			_ = os.Rename(old, target)
		}
	}()
	return filepath.WalkDir(staged.Dir, func(source string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(staged.Dir, source)
		if err != nil || relative == "." {
			return err
		}
		target := filepath.Join(installDir, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if _, statErr := os.Lstat(target); statErr == nil {
			old := target + oldSuffix
			os.Remove(old)
			if err := os.Rename(target, old); err != nil {
				return fmt.Errorf("could not replace %s: %w", relative, err)
			}
			replaced = append(replaced, [2]string{target, old})
		}
		if err := moveFile(source, target); err != nil {
			return fmt.Errorf("could not install %s: %w", relative, err)
		}
		return nil
	})
}

// moveFile renames source to target, or copies it when they are on
// different volumes.
func moveFile(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm()|0o600)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(target)
		return err
	}
	return os.Remove(source)
}

// CleanupOld removes the files Apply renamed away, once the program that
// used them has exited. Files still in use are left for the next start.
func CleanupOld(installDir string) {
	_ = filepath.WalkDir(installDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), oldSuffix) {
			_ = os.Remove(path)
		}
		return nil
	})
}
