// Command release packs the built app (build/bin) into the archive a GitHub
// release carries for this OS and CPU, named as internal/update expects
// (Mayak-0.1.6-windows-amd64.zip, Mayak-0.1.6-darwin-arm64.tar.gz, ...), and writes its
// SHA-256 beside it. The release workflow joins those into SHA256SUMS.txt.
//
// Windows archives hold Mayak.exe, the bundled Tesseract runtime and the
// third-party notices; the others hold the binary and the notices. The
// files sit at the archive's root, so the updater unpacks them straight
// over the installed ones.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/local/mayak/internal/update"
)

func main() {
	log.SetFlags(0)
	bin := flag.String("bin", "build/bin", "directory task build wrote the app to")
	out := flag.String("out", "build/dist", "directory to write the archive and its checksum to")
	goos := flag.String("os", runtime.GOOS, "target OS (windows, darwin, linux)")
	goarch := flag.String("arch", runtime.GOARCH, "target CPU (amd64, arm64)")
	version := flag.String("version", "0.0.0", "the app's version, for the macOS bundle")
	icon := flag.String("icon", "build/appicon.png", "the app icon, for the macOS bundle")
	dmg := flag.Bool("dmg", true, "on macOS, also build Mayak.app and a disk image (needs hdiutil)")
	flag.Parse()
	name := update.ArchiveName(*version, *goos, *goarch)
	entries := []string{"THIRD_PARTY_NOTICES.txt"}
	if *goos == "windows" {
		entries = append(entries, "Mayak.exe", "tesseract")
	} else {
		entries = append(entries, "Mayak")
	}
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(*bin, entry)); err != nil {
			log.Fatalf("%s is missing from %s: run `task build` first", entry, *bin)
		}
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	target := filepath.Join(*out, name)
	var err error
	if strings.HasSuffix(name, ".zip") {
		err = writeZip(target, *bin, entries)
	} else {
		err = writeTarGz(target, *bin, entries)
	}
	if err != nil {
		os.Remove(target)
		log.Fatal(err)
	}
	outputs := []string{target}
	// macOS: Mayak.app in a disk image, for people; the tar.gz above stays
	// for the updater.
	if *goos == "darwin" {
		staging := filepath.Join(*out, "dmg-"+*goarch)
		os.RemoveAll(staging)
		if _, err := buildApp(staging, *bin, versionCore(*version), *icon); err != nil {
			log.Fatal(err)
		}
		if *dmg {
			image := filepath.Join(*out, update.ImageName(*version, *goarch))
			if err := buildDMG(staging, image); err != nil {
				log.Fatal(err)
			}
			outputs = append(outputs, image)
			os.RemoveAll(staging)
		}
	}
	// The installer (tools/nsis), when it was built, gets its checksum too.
	installers, _ := filepath.Glob(filepath.Join(*out, "Mayak-Setup-*.exe"))
	for _, path := range append(outputs, installers...) {
		sum, err := fileSHA256(path)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path+".sha256", []byte(sum+"  "+filepath.Base(path)+"\n"), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s  %s\n", sum, path)
	}
}

// versionCore is the numeric part of the version ("v0.1.2-3-gabcdef0" is
// "0.1.2"), which the bundle's version fields want.
func versionCore(value string) string {
	parsed, err := update.ParseSemver(value)
	if err != nil {
		return "0.0.0"
	}
	return fmt.Sprintf("%d.%d.%d", parsed.Major, parsed.Minor, parsed.Patch)
}

// walk calls visit for every regular file under the entries, with the
// path the file has inside the archive (forward slashes).
func walk(bin string, entries []string, visit func(archivePath, path string, info fs.FileInfo) error) error {
	for _, entry := range entries {
		root := filepath.Join(bin, entry)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			relative, err := filepath.Rel(bin, path)
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			return visit(filepath.ToSlash(relative), path, info)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func writeZip(target, bin string, entries []string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	err = walk(bin, entries, func(archivePath, path string, info fs.FileInfo) error {
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = archivePath
		header.Method = zip.Deflate
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		return copyFile(entry, path)
	})
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func writeTarGz(target, bin string, entries []string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(file)
	writer := tar.NewWriter(gz)
	err = walk(bin, entries, func(archivePath, path string, info fs.FileInfo) error {
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = archivePath
		header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
		// The app must stay executable after unpacking, whatever the
		// build host's umask.
		if archivePath == "Mayak" {
			header.Mode = 0o755
		}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		return copyFile(writer, path)
	})
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if closeErr := gz.Close(); err == nil {
		err = closeErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func copyFile(to io.Writer, path string) error {
	from, err := os.Open(path)
	if err != nil {
		return err
	}
	defer from.Close()
	_, err = io.Copy(to, from)
	return err
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
