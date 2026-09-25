// Command nsis builds the Windows installer (build/windows/nsis/mayak.nsi)
// from the app in build/bin. It uses makensis from PATH or the usual NSIS
// install, and otherwise fetches the portable NSIS release once
// (checksum-verified) into build/nsis-cache.
//
//	go run ./tools/nsis -version v0.1.0 -bin build/bin -out build/dist
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/local/mayak/internal/update"
)

// The portable NSIS release (SourceForge; the checksum pins it).
const (
	nsisVersion = "3.12"
	nsisURL     = "https://downloads.sourceforge.net/project/nsis/NSIS%203/" + nsisVersion + "/nsis-" + nsisVersion + ".zip"
	nsisSHA256  = "56581f90db321581c5381193d796fffcf2d24b2f8fed2160a6c6a3baa67f2c4f"
)

func main() {
	log.SetFlags(0)
	version := flag.String("version", "dev", "the app's version (v1.2.3, 1.2.3-4-gabcdef0 or dev)")
	bin := flag.String("bin", "build/bin", "directory task build wrote the app to")
	out := flag.String("out", "build/dist", "directory to write the installer to")
	script := flag.String("script", "build/windows/nsis/mayak.nsi", "the NSIS script")
	icon := flag.String("icon", "build/windows/icon.ico", "the installer's icon")
	license := flag.String("license", "LICENSE", "the license file to install as LICENSE.txt")
	cache := flag.String("cache", "build/nsis-cache", "where to keep a fetched NSIS")
	flag.Parse()

	for _, entry := range []string{"Mayak.exe", "THIRD_PARTY_NOTICES.txt", "tesseract"} {
		if _, err := os.Stat(filepath.Join(*bin, entry)); err != nil {
			log.Fatalf("%s is missing from %s: run `task build` first", entry, *bin)
		}
	}
	makensis, err := findMakensis(*cache)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	core, display := versions(*version)
	target := filepath.Join(*out, update.InstallerName(*version))
	defines := map[string]string{
		"VERSION":        core,
		"VERSION4":       core + ".0",
		"DISPLAYVERSION": display,
		"BIN":            absolute(*bin),
		"OUTFILE":        absolute(target),
		"ICON":           absolute(*icon),
		"LICENSE":        absolute(*license),
	}
	args := []string{"/V2"}
	for name, value := range defines {
		args = append(args, "/D"+name+"="+value)
	}
	args = append(args, absolute(*script))
	cmd := exec.Command(makensis, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("makensis: %v", err)
	}
	fmt.Println(target)
}

// versions splits the version into the numeric core the installer's version
// resource needs ("0.1.0") and the text shown in Apps & features.
func versions(value string) (core, display string) {
	display = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parsed, err := update.ParseSemver(display)
	if err != nil {
		return "0.0.0", display
	}
	return fmt.Sprintf("%d.%d.%d", parsed.Major, parsed.Minor, parsed.Patch), display
}

func absolute(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatal(err)
	}
	return abs
}

// findMakensis returns makensis: from PATH, from the standard NSIS install,
// or from the portable release fetched into cache.
func findMakensis(cache string) (string, error) {
	if path, err := exec.LookPath("makensis"); err == nil {
		return path, nil
	}
	for _, path := range []string{`C:\Program Files (x86)\NSIS\makensis.exe`, `C:\Program Files\NSIS\makensis.exe`} {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	cached := filepath.Join(cache, "nsis-"+nsisVersion, "makensis.exe")
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}
	log.Printf("makensis not found; fetching NSIS %s into %s", nsisVersion, cache)
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return "", err
	}
	archive := filepath.Join(cache, "nsis-"+nsisVersion+".zip")
	if err := download(nsisURL, archive, nsisSHA256); err != nil {
		return "", err
	}
	if err := update.Extract(archive, filepath.Join(cache, "unpacked")); err != nil {
		return "", err
	}
	if err := os.Rename(filepath.Join(cache, "unpacked", "nsis-"+nsisVersion), filepath.Join(cache, "nsis-"+nsisVersion)); err != nil {
		return "", err
	}
	if _, err := os.Stat(cached); err != nil {
		return "", fmt.Errorf("NSIS archive did not contain %s", cached)
	}
	return cached, nil
}

func download(url, target, sum string) error {
	if data, err := os.ReadFile(target); err == nil {
		if digest := sha256.Sum256(data); hex.EncodeToString(digest[:]) == sum {
			return nil
		}
	}
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return err
	}
	if digest := sha256.Sum256(data); hex.EncodeToString(digest[:]) != sum {
		return fmt.Errorf("%s: checksum mismatch (got %x)", url, digest)
	}
	return os.WriteFile(target, data, 0o644)
}
