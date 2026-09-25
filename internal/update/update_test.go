package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func zipArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		entry.Write([]byte(content))
	}
	writer.Close()
	return buffer.Bytes()
}

func tarGzArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gz)
	for name, content := range files {
		mode := int64(0o644)
		if !strings.Contains(name, ".") {
			mode = 0o755
		}
		writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(content)), Typeflag: tar.TypeReg})
		writer.Write([]byte(content))
	}
	writer.Close()
	gz.Close()
	return buffer.Bytes()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestExtractZipAndTarGz(t *testing.T) {
	files := map[string]string{"Mayak.exe": "new exe", "tesseract/tesseract.exe": "ocr", "THIRD_PARTY_NOTICES.txt": "notices"}
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "a.zip")
	os.WriteFile(zipPath, zipArchive(t, files), 0o644)
	out := filepath.Join(dir, "zip-out")
	if err := Extract(zipPath, out); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(out, "tesseract", "tesseract.exe")); got != "ocr" {
		t.Errorf("zip entry: %q", got)
	}
	tarPath := filepath.Join(dir, "a.tar.gz")
	os.WriteFile(tarPath, tarGzArchive(t, map[string]string{"Mayak": "bin", "THIRD_PARTY_NOTICES.txt": "notices"}), 0o644)
	out = filepath.Join(dir, "tar-out")
	if err := Extract(tarPath, out); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(out, "Mayak")); got != "bin" {
		t.Errorf("tar entry: %q", got)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(filepath.Join(out, "Mayak")); info.Mode().Perm()&0o100 == 0 {
			t.Errorf("executable bit lost: %v", info.Mode())
		}
	}
}

func TestExtractRejectsEscapingEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"../evil.txt", "sub/../../evil.txt"} {
		path := filepath.Join(dir, "bad.zip")
		os.WriteFile(path, zipArchive(t, map[string]string{name: "x"}), 0o644)
		if err := Extract(path, filepath.Join(dir, "out")); err == nil {
			t.Errorf("%q extracted", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
		t.Error("file written outside the target")
	}
}

func TestApplyReplacesFilesAndCleanupRemovesOld(t *testing.T) {
	install := t.TempDir()
	os.WriteFile(filepath.Join(install, "Mayak.exe"), []byte("old exe"), 0o644)
	os.WriteFile(filepath.Join(install, "settings.local"), []byte("keep"), 0o644)
	os.MkdirAll(filepath.Join(install, "tesseract"), 0o755)
	os.WriteFile(filepath.Join(install, "tesseract", "old.dll"), []byte("old dll"), 0o644)
	staged := Staged{Version: "0.2.0", Dir: filepath.Join(t.TempDir(), "files")}
	os.MkdirAll(filepath.Join(staged.Dir, "tesseract"), 0o755)
	os.WriteFile(filepath.Join(staged.Dir, "Mayak.exe"), []byte("new exe"), 0o755)
	os.WriteFile(filepath.Join(staged.Dir, "tesseract", "old.dll"), []byte("new dll"), 0o644)
	os.WriteFile(filepath.Join(staged.Dir, "tesseract", "new.dll"), []byte("added"), 0o644)
	if err := Apply(staged, install); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"Mayak.exe": "new exe", "settings.local": "keep", "tesseract/old.dll": "new dll", "tesseract/new.dll": "added", "Mayak.exe" + oldSuffix: "old exe", "tesseract/old.dll" + oldSuffix: "old dll"} {
		if got := readFile(t, filepath.Join(install, filepath.FromSlash(path))); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
	CleanupOld(install)
	for _, path := range []string{"Mayak.exe" + oldSuffix, "tesseract/old.dll" + oldSuffix} {
		if _, err := os.Stat(filepath.Join(install, filepath.FromSlash(path))); err == nil {
			t.Errorf("%s still there", path)
		}
	}
	if got := readFile(t, filepath.Join(install, "Mayak.exe")); got != "new exe" {
		t.Errorf("cleanup touched the new file: %q", got)
	}
}

func TestApplyRestoresOnFailure(t *testing.T) {
	install := t.TempDir()
	os.WriteFile(filepath.Join(install, "a.txt"), []byte("old a"), 0o644)
	// b is a file in the install but a directory in the update: it cannot
	// be created.
	os.WriteFile(filepath.Join(install, "b"), []byte("old b"), 0o644)
	staged := Staged{Version: "0.2.0", Dir: t.TempDir()}
	os.WriteFile(filepath.Join(staged.Dir, "a.txt"), []byte("new a"), 0o644)
	os.MkdirAll(filepath.Join(staged.Dir, "b"), 0o755)
	os.WriteFile(filepath.Join(staged.Dir, "b", "inside.txt"), []byte("new b"), 0o644)
	if err := Apply(staged, install); err == nil {
		t.Fatal("apply succeeded")
	}
	if got := readFile(t, filepath.Join(install, "a.txt")); got != "old a" {
		t.Errorf("a.txt not restored: %q", got)
	}
	if _, err := os.Stat(filepath.Join(install, "a.txt"+oldSuffix)); err == nil {
		t.Error("old copy left after the restore")
	}
}

func TestStageFromServer(t *testing.T) {
	files := map[string]string{"Mayak": "new binary", "THIRD_PARTY_NOTICES.txt": "notices"}
	var archive []byte
	if runtime.GOOS == "windows" {
		archive = zipArchive(t, files)
	} else {
		archive = tarGzArchive(t, files)
	}
	name := ArchiveName(runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(archive)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/ichi0g0y/mayak/releases/latest":
			if r.Header.Get("User-Agent") != "MAYAK/test" {
				t.Errorf("User-Agent %q", r.Header.Get("User-Agent"))
			}
			json.NewEncoder(w).Encode(Release{Tag: "v0.2.0", Name: "MAYAK 0.2.0", URL: "https://example.test/release", PublishedAt: time.Now(), Assets: []Asset{
				{Name: name, URL: server.URL + "/download/" + name, Size: int64(len(archive))},
				{Name: ChecksumsAsset, URL: server.URL + "/download/" + ChecksumsAsset},
			}})
		case "/download/" + name:
			w.Write(archive)
		case "/download/" + ChecksumsAsset:
			w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + name + "\n0000000000000000000000000000000000000000000000000000000000000000  other.zip\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient("MAYAK/test")
	client.APIBase = server.URL
	release, err := client.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if release.Version() != "0.2.0" || !IsNewer("0.1.0", release.Tag) {
		t.Fatalf("release %+v", release)
	}
	staging := filepath.Join(t.TempDir(), "updates")
	var last int64
	staged, err := client.Stage(context.Background(), release, staging, func(done, total int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	if last != int64(len(archive)) {
		t.Errorf("progress stopped at %d of %d", last, len(archive))
	}
	if got := readFile(t, filepath.Join(staged.Dir, "Mayak")); got != "new binary" {
		t.Errorf("staged binary: %q", got)
	}
	loaded, ok := LoadStaged(staging)
	if !ok || loaded.Version != "0.2.0" || loaded.Dir != staged.Dir {
		t.Errorf("LoadStaged: %+v %v", loaded, ok)
	}
	if _, err := os.Stat(filepath.Join(staging, name)); err == nil {
		t.Error("archive kept after unpacking")
	}
	if err := Discard(staging); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadStaged(staging); ok {
		t.Error("staged after discard")
	}
}

func TestDownloadRejectsChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("content")) }))
	defer server.Close()
	client := NewClient("MAYAK/test")
	dir := t.TempDir()
	_, err := client.Download(context.Background(), Asset{Name: "a.zip", URL: server.URL}, strings.Repeat("0", 64), dir, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("leftover files: %v", entries)
	}
	if _, err := client.Download(context.Background(), Asset{Name: "a.zip", URL: server.URL}, "", dir, nil); err == nil {
		t.Error("download without a checksum succeeded")
	}
}

func TestParseChecksums(t *testing.T) {
	sums := ParseChecksums("abc  x\n" + strings.Repeat("A", 64) + " *Mayak-windows-amd64.zip\n" + strings.Repeat("b", 64) + "  Mayak-darwin-arm64.tar.gz\n")
	if sums["Mayak-windows-amd64.zip"] != strings.Repeat("a", 64) || sums["Mayak-darwin-arm64.tar.gz"] != strings.Repeat("b", 64) || len(sums) != 2 {
		t.Errorf("%v", sums)
	}
}

func TestWaitForPreviousInstance(t *testing.T) {
	if WaitForPreviousInstance(time.Second) {
		t.Error("waited without the variable")
	}
	t.Setenv(waitEnv, "999999999")
	start := time.Now()
	if !WaitForPreviousInstance(2 * time.Second) {
		t.Error("did not report a restart")
	}
	if time.Since(start) > time.Second {
		t.Error("waited for a process that does not exist")
	}
	if os.Getenv(waitEnv) != "" {
		t.Error("variable left for child processes")
	}
	t.Setenv(waitEnv, "x")
	if WaitForPreviousInstance(time.Second) {
		t.Error("accepted a non-numeric pid")
	}
}
