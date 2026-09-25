// Command tessbundle builds the Tesseract runtime that ships next to
// Mayak.exe from an extracted UB Mannheim Windows build:
//
//	tessbundle -fetch -out build/bin/tesseract -lang eng,jpn
//	tessbundle -src <extracted installer> -out build/bin/tesseract [-lang eng,jpn] [-tessdata extra.traineddata...]
//
// It copies tesseract.exe and only the DLLs it (transitively) imports, strips
// the DWARF debug sections those builds carry (libtesseract-5.dll shrinks from
// about 100 MB to 3 MB), and adds the chosen traineddata and the licenses.
package main

import (
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	src := flag.String("src", "", "extracted UB Mannheim Tesseract directory")
	fetch := flag.Bool("fetch", false, "download and extract the pinned UB Mannheim build into -cache instead of using -src")
	cache := flag.String("cache", "build/tesseract-cache", "download cache for -fetch")
	sevenZip := flag.String("7z", `C:\Program Files\7-Zip\7z.exe`, "7-Zip, used by -fetch to unpack the installer without running it")
	out := flag.String("out", "build/bin/tesseract", "output directory")
	langs := flag.String("lang", "eng", "comma-separated traineddata from -src/tessdata to include")
	extra := flag.String("tessdata", "", "comma-separated extra .traineddata files (e.g. MAYAK' own models)")
	flag.Parse()
	if *fetch {
		dir, err := fetchRuntime(*cache, *sevenZip)
		if err != nil {
			log.Fatal(err)
		}
		*src = dir
	}
	if *src == "" {
		log.Fatal("-src or -fetch is required")
	}
	if err := os.RemoveAll(*out); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(*out, "tessdata"), 0o755); err != nil {
		log.Fatal(err)
	}
	files, err := dependencies(*src, "tesseract.exe")
	if err != nil {
		log.Fatal(err)
	}
	var total int64
	for _, name := range files {
		size, err := copyStripped(filepath.Join(*src, name), filepath.Join(*out, name))
		if err != nil {
			log.Fatalf("%s: %v", name, err)
		}
		total += size
	}
	for _, lang := range strings.Split(*langs, ",") {
		if lang = strings.TrimSpace(lang); lang != "" {
			must(copyFile(filepath.Join(*src, "tessdata", lang+".traineddata"), filepath.Join(*out, "tessdata", lang+".traineddata")))
		}
	}
	for _, file := range strings.Split(*extra, ",") {
		if file = strings.TrimSpace(file); file != "" {
			must(copyFile(file, filepath.Join(*out, "tessdata", filepath.Base(file))))
		}
	}
	must(copyFile(filepath.Join(*src, "doc", "LICENSE"), filepath.Join(*out, "LICENSE.tesseract.txt")))
	must(os.WriteFile(filepath.Join(*out, "README.txt"), []byte(readme), 0o644))
	fmt.Printf("%d files, %.1f MB of binaries in %s\n", len(files), float64(total)/(1<<20), *out)
}

const readme = `Tesseract OCR runtime bundled with MAYAK.

Tesseract is licensed under the Apache License 2.0 (LICENSE.tesseract.txt).
The binaries come from the UB Mannheim Windows build
(https://github.com/UB-Mannheim/tesseract) with debug information removed.
The libraries they use keep their own open source licenses.
`

// Pinned downloads. Updating Tesseract means changing these together.
var downloads = []struct{ url, file, sha256 string }{
	{"https://github.com/UB-Mannheim/tesseract/releases/download/v5.4.0.20240606/tesseract-ocr-w64-setup-5.4.0.20240606.exe", "tesseract-setup.exe", "c885fff6998e0608ba4bb8ab51436e1c6775c2bafc2559a19b423e18678b60c9"},
	{"https://raw.githubusercontent.com/tesseract-ocr/tessdata_fast/80d92b7db61cb6ddc519049990be34e4c913566f/jpn.traineddata", "jpn.traineddata", "1f5de9236d2e85f5fdf4b3c500f2d4926f8d9449f28f5394472d9e8d83b91b4d"},
}

// fetchRuntime downloads the pinned files into cache (once), verifies them and
// unpacks the installer with 7-Zip. It returns the unpacked directory, which
// also receives the extra traineddata.
func fetchRuntime(cache, sevenZip string) (string, error) {
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return "", err
	}
	for _, d := range downloads {
		path := filepath.Join(cache, d.file)
		if sum, err := fileSHA256(path); err == nil && sum == d.sha256 {
			continue
		}
		fmt.Println("downloading", d.url)
		if err := download(d.url, path); err != nil {
			return "", err
		}
		if sum, err := fileSHA256(path); err != nil || sum != d.sha256 {
			os.Remove(path)
			return "", fmt.Errorf("%s: checksum mismatch", d.file)
		}
	}
	dir := filepath.Join(cache, "extracted")
	if _, err := os.Stat(filepath.Join(dir, "tesseract.exe")); err != nil {
		cmd := exec.Command(sevenZip, "x", "-y", "-o"+dir, filepath.Join(cache, "tesseract-setup.exe"))
		cmd.Stdout, cmd.Stderr = io.Discard, os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("7-Zip could not unpack the installer (is it installed at %s?): %w", sevenZip, err)
		}
	}
	if err := copyFile(filepath.Join(cache, "jpn.traineddata"), filepath.Join(dir, "tessdata", "jpn.traineddata")); err != nil {
		return "", err
	}
	return dir, nil
}

func download(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err = io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// dependencies returns root and every DLL it imports, directly or through
// other DLLs, that exists in dir. System DLLs are not in dir and are skipped.
func dependencies(dir, root string) ([]string, error) {
	seen := map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		key := strings.ToLower(name)
		if seen[key] {
			return nil
		}
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			return nil
		}
		seen[key] = true
		imports, err := importedLibraries(path)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		for _, lib := range imports {
			if err := visit(lib); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(seen))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if seen[strings.ToLower(e.Name())] {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// importedLibraries reads the DLL names of a PE32+ import directory.
// debug/pe's ImportedLibraries returns nothing for these MinGW builds.
func importedLibraries(path string) ([]string, error) {
	f, err := pe.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opt, ok := f.OptionalHeader.(*pe.OptionalHeader64)
	if !ok || len(opt.DataDirectory) <= pe.IMAGE_DIRECTORY_ENTRY_IMPORT {
		return nil, errors.New("only PE32+ is supported")
	}
	dir := opt.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_IMPORT]
	if dir.VirtualAddress == 0 {
		return nil, nil
	}
	// read returns the bytes at a virtual address, from the section holding it.
	read := func(rva uint32) []byte {
		for _, s := range f.Sections {
			if rva >= s.VirtualAddress && rva < s.VirtualAddress+s.Size {
				data, err := s.Data()
				if err != nil {
					return nil
				}
				return data[rva-s.VirtualAddress:]
			}
		}
		return nil
	}
	var libs []string
	for rva := dir.VirtualAddress; ; rva += 20 {
		desc := read(rva)
		if len(desc) < 20 {
			return nil, errors.New("truncated import directory")
		}
		nameRVA := binary.LittleEndian.Uint32(desc[12:])
		if nameRVA == 0 {
			return libs, nil
		}
		name := read(nameRVA)
		if end := strings.IndexByte(string(name), 0); end > 0 {
			libs = append(libs, string(name[:end]))
		}
	}
}

// copyStripped copies a PE file without trailing .debug_* sections and COFF
// symbols, which the loader never maps. Files without them are copied as is.
func copyStripped(src, dst string) (int64, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return 0, err
	}
	stripped, err := stripDebug(data)
	if err != nil {
		return 0, err
	}
	return int64(len(stripped)), os.WriteFile(dst, stripped, 0o755)
}

func stripDebug(data []byte) ([]byte, error) {
	le := binary.LittleEndian
	if len(data) < 0x40 || string(data[:2]) != "MZ" {
		return nil, errors.New("not a PE file")
	}
	peOff := int(le.Uint32(data[0x3c:]))
	if peOff+24 > len(data) || string(data[peOff:peOff+4]) != "PE\x00\x00" {
		return nil, errors.New("not a PE file")
	}
	fh := peOff + 4
	numSections := int(le.Uint16(data[fh+2:]))
	optSize := int(le.Uint16(data[fh+16:]))
	opt := fh + 20
	if le.Uint16(data[opt:]) != 0x20b {
		return nil, errors.New("only PE32+ is supported")
	}
	sectionAlign := le.Uint32(data[opt+32:])
	headers := opt + optSize

	keep := numSections
	for i := 0; i < numSections; i++ {
		name := strings.TrimRight(string(data[headers+i*40:headers+i*40+8]), "\x00")
		if strings.HasPrefix(name, ".debug") || strings.HasPrefix(name, "/") {
			keep = i
			break
		}
	}
	if keep == numSections {
		return data, nil
	}
	// Everything from the first debug section on must be debug data at the end
	// of the file; otherwise truncating would cut something the loader needs.
	cut := len(data)
	var imageEnd uint32
	for i := 0; i < numSections; i++ {
		h := data[headers+i*40:]
		vsize, va, rawSize, rawPtr := le.Uint32(h[8:]), le.Uint32(h[12:]), le.Uint32(h[16:]), le.Uint32(h[20:])
		if i >= keep {
			name := strings.TrimRight(string(h[:8]), "\x00")
			if !strings.HasPrefix(name, ".debug") && !strings.HasPrefix(name, "/") {
				return nil, fmt.Errorf("section %q follows debug sections", name)
			}
			if rawSize > 0 && int(rawPtr) < cut {
				cut = int(rawPtr)
			}
			continue
		}
		if va+vsize > imageEnd {
			imageEnd = va + vsize
		}
	}
	for i := 0; i < keep; i++ {
		h := data[headers+i*40:]
		if rawSize, rawPtr := le.Uint32(h[16:]), le.Uint32(h[20:]); rawSize > 0 && int(rawPtr+rawSize) > cut {
			return nil, errors.New("kept section overlaps debug data")
		}
	}
	out := append([]byte(nil), data[:cut]...)
	le.PutUint16(out[fh+2:], uint16(keep))
	le.PutUint32(out[fh+8:], 0)  // PointerToSymbolTable
	le.PutUint32(out[fh+12:], 0) // NumberOfSymbols
	le.PutUint32(out[opt+56:], (imageEnd+sectionAlign-1)/sectionAlign*sectionAlign)
	le.PutUint32(out[opt+64:], 0) // CheckSum is not verified for user-mode images
	clear(out[headers+keep*40 : headers+numSections*40])
	return out, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
