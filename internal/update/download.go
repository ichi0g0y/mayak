package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// maxArchiveBytes bounds a release archive and its unpacked files.
const maxArchiveBytes = 1 << 30

// Progress reports the bytes downloaded so far and the total (0 when unknown).
type Progress func(done, total int64)

// Download fetches an asset into dir and verifies it against sum (lower-case
// hex SHA-256). It returns the file's path.
func (c *Client) Download(ctx context.Context, asset Asset, sum, dir string, progress Progress) (string, error) {
	if sum == "" {
		return "", fmt.Errorf("%s: no checksum published", asset.Name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, asset.Name)
	partial := target + ".part"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	// The archive takes longer than an API call; the context bounds it.
	client := *c.HTTP
	client.Timeout = 0
	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", asset.Name, response.Status)
	}
	total := response.ContentLength
	if total <= 0 {
		total = asset.Size
	}
	file, err := os.Create(partial)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	var done int64
	buffer := make([]byte, 256<<10)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			done += int64(n)
			if done > maxArchiveBytes {
				readErr = fmt.Errorf("%s: larger than %d bytes", asset.Name, maxArchiveBytes)
			} else if _, err = file.Write(buffer[:n]); err == nil {
				digest.Write(buffer[:n])
				if progress != nil {
					progress(done, total)
				}
			}
		}
		if err == nil && readErr != nil && !errors.Is(readErr, io.EOF) {
			err = readErr
		}
		if err != nil || errors.Is(readErr, io.EOF) {
			break
		}
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(partial)
		return "", err
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != strings.ToLower(sum) {
		os.Remove(partial)
		return "", fmt.Errorf("%s: checksum mismatch (got %s, want %s)", asset.Name, got, sum)
	}
	os.Remove(target)
	if err := os.Rename(partial, target); err != nil {
		os.Remove(partial)
		return "", err
	}
	return target, nil
}

// Extract unpacks a .zip or .tar.gz archive into dir, which it empties
// first. Entries are kept inside dir; links and other special entries are
// skipped.
func Extract(archive, dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	switch {
	case strings.HasSuffix(archive, ".zip"):
		return extractZip(archive, dir)
	case strings.HasSuffix(archive, ".tar.gz"), strings.HasSuffix(archive, ".tgz"):
		return extractTarGz(archive, dir)
	}
	return fmt.Errorf("%s: unknown archive format", filepath.Base(archive))
}

func extractZip(archive, dir string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	var total uint64
	for _, entry := range reader.File {
		total += entry.UncompressedSize64
		if total > maxArchiveBytes {
			return fmt.Errorf("archive unpacks to more than %d bytes", maxArchiveBytes)
		}
		target, err := insideDir(dir, entry.Name)
		if err != nil {
			return err
		}
		mode := entry.Mode()
		if mode.IsDir() || strings.HasSuffix(entry.Name, "/") {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if !mode.IsRegular() {
			continue
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		err = writeEntry(target, source, mode.Perm())
		source.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(archive, dir string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer unzipped.Close()
	reader := tar.NewReader(unzipped)
	var total int64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := insideDir(dir, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			total += header.Size
			if total > maxArchiveBytes {
				return fmt.Errorf("archive unpacks to more than %d bytes", maxArchiveBytes)
			}
			if err := writeEntry(target, reader, os.FileMode(header.Mode).Perm()); err != nil {
				return err
			}
		}
	}
}

// insideDir resolves an archive entry's name under dir and rejects names
// that would leave it.
func insideDir(dir, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.VolumeName(clean) != "" {
		return "", fmt.Errorf("archive entry outside the archive: %q", name)
	}
	return filepath.Join(dir, clean), nil
}

func writeEntry(target string, source io.Reader, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	// Zip archives built on Windows carry no permissions; keep files
	// readable and let the platform's launcher decide (see Apply).
	if perm == 0 {
		perm = 0o644
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm|0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, io.LimitReader(source, maxArchiveBytes))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
