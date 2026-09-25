// Command notices writes THIRD_PARTY_NOTICES.txt: the licenses of everything
// linked into Mayak.exe or shipped beside it (the Go modules in the build,
// the frontend's dependencies, the bundled OCR data) and the sources of the
// filter lists the browser downloads.
//
//	go run ./tools/notices -out build/bin/THIRD_PARTY_NOTICES.txt
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var licenseFile = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice)([-_.].*)?$`)

type entry struct {
	name, version, license, text string
}

func main() {
	out := flag.String("out", "build/bin/THIRD_PARTY_NOTICES.txt", "file to write")
	flag.Parse()
	var entries []entry
	entries = append(entries, goModules()...)
	entries = append(entries, entry{
		name: "WebView2Loader (github.com/wailsapp/go-webview2/webviewloader)", version: "",
		text: readFile("third_party/go-webview2/webviewloader/LICENSE"),
	})
	entries = append(entries, npmPackages()...)
	entries = append(entries, bundledData...)

	var b strings.Builder
	b.WriteString("Third-party notices for MAYAK\n\n")
	b.WriteString("MAYAK is licensed under the GNU General Public License v3.0 (see LICENSE).\n")
	b.WriteString("It links or ships the following works, under their own licenses.\n")
	b.WriteString("The Tesseract OCR runtime in the tesseract folder carries its own notices\n")
	b.WriteString("(README.txt, LICENSE.tesseract.txt).\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "\n%s\n== %s", strings.Repeat("=", 78), e.name)
		if e.version != "" {
			fmt.Fprintf(&b, " %s", e.version)
		}
		if e.license != "" {
			fmt.Fprintf(&b, " (%s)", e.license)
		}
		b.WriteString("\n\n")
		b.WriteString(strings.TrimRight(e.text, "\n"))
		b.WriteString("\n")
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("%d notices in %s\n", len(entries), *out)
}

// goModules lists the modules whose packages are linked into the Windows
// binary, with their license files. A module without one stops the build:
// it must be looked at, not left out.
func goModules() []entry {
	cmd := exec.Command("go", "list", "-deps", "-json=Module,Standard", ".")
	cmd.Env = append(os.Environ(), "GOOS=windows")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		fail(fmt.Errorf("go list: %w", err))
	}
	type pkg struct {
		Standard bool
		Module   *struct {
			Path, Version, Dir string
			Main               bool
			Replace            *struct{ Path, Dir string }
		}
	}
	seen := map[string]bool{}
	var entries []entry
	dec := json.NewDecoder(bytes.NewReader(output))
	for {
		var p pkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			fail(err)
		}
		m := p.Module
		if p.Standard || m == nil || m.Main || seen[m.Path] {
			continue
		}
		seen[m.Path] = true
		dir := m.Dir
		if m.Replace != nil && m.Replace.Dir != "" {
			dir = m.Replace.Dir
		}
		text := licenseTexts(dir)
		if text == "" {
			fail(fmt.Errorf("no license file in %s (%s)", m.Path, dir))
		}
		name := m.Path
		if m.Replace != nil {
			name += " (MAYAK's copy in " + filepath.ToSlash(m.Replace.Path) + ")"
		}
		entries = append(entries, entry{name: name, version: m.Version, text: text})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return entries
}

// npmPackages walks the frontend's runtime dependencies (not the build
// tools) through the installed node_modules.
func npmPackages() []entry {
	type manifest struct {
		Name, Version        string
		License              json.RawMessage
		Dependencies         map[string]string
		OptionalDependencies map[string]string
	}
	read := func(path string) manifest {
		var m manifest
		if err := json.Unmarshal([]byte(readFile(path)), &m); err != nil {
			fail(fmt.Errorf("%s: %w", path, err))
		}
		return m
	}
	root := read("frontend/package.json")
	queue := make([]string, 0, len(root.Dependencies))
	for name := range root.Dependencies {
		queue = append(queue, name)
	}
	seen := map[string]bool{}
	var entries []entry
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		dir := filepath.Join("frontend", "node_modules", filepath.FromSlash(name))
		m := read(filepath.Join(dir, "package.json"))
		for dep := range m.Dependencies {
			queue = append(queue, dep)
		}
		for dep := range m.OptionalDependencies {
			if _, err := os.Stat(filepath.Join("frontend", "node_modules", filepath.FromSlash(dep))); err == nil {
				queue = append(queue, dep)
			}
		}
		license := strings.Trim(string(m.License), `"`)
		if strings.HasPrefix(license, "{") {
			var obj struct{ Type string }
			_ = json.Unmarshal(m.License, &obj)
			license = obj.Type
		}
		text := licenseTexts(dir)
		if text == "" {
			if license == "" {
				fail(fmt.Errorf("no license in %s", dir))
			}
			text = "Licensed under " + license + " (no license file in the package)."
		}
		entries = append(entries, entry{name: name, version: m.Version, license: license, text: text})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return entries
}

// bundledData is what ships beside the binary or is fetched by it, outside
// the two package managers.
var bundledData = []entry{
	{
		name: "Tesseract OCR runtime", license: "Apache-2.0",
		text: "tesseract.exe and its libraries come from the UB Mannheim build\n" +
			"(https://github.com/UB-Mannheim/tesseract). Their notices are in the\n" +
			"tesseract folder (README.txt, LICENSE.tesseract.txt).",
	},
	{
		name: "jpn.traineddata (tesseract-ocr/tessdata_fast)", license: "Apache-2.0",
		text: "Copyright (C) The Tesseract OCR project authors.\n" +
			"https://github.com/tesseract-ocr/tessdata_fast\n" +
			"Licensed under the Apache License, Version 2.0 (LICENSE.tesseract.txt).",
	},
	{
		name: "eft.traineddata and eftjpn.traineddata", license: "Apache-2.0",
		text: "MAYAK's models, fine-tuned from the Tesseract eng and jpn models of\n" +
			"https://github.com/tesseract-ocr/tessdata_best (Apache License, Version 2.0).",
	},
	{
		name: "EasyList and EasyPrivacy", license: "GPL-3.0-or-later or CC-BY-SA-3.0",
		text: "The built-in browser downloads these filter lists at first use; they are\n" +
			"not distributed with MAYAK. Copyright (C) The EasyList authors,\n" +
			"https://easylist.to/, dual-licensed under the GNU General Public License\n" +
			"version 3 or later and Creative Commons Attribution-ShareAlike 3.0.",
	},
	{
		name: "AdGuard Japanese filter", license: "GPL-3.0",
		text: "The built-in browser downloads this filter list at first use; it is not\n" +
			"distributed with MAYAK. Copyright (C) AdGuard Software Ltd,\n" +
			"https://github.com/AdguardTeam/AdguardFilters, GNU General Public License v3.0.",
	},
}

// licenseTexts joins the license files found directly in dir.
func licenseTexts(dir string) string {
	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var parts []string
	for _, f := range files {
		if !f.IsDir() && licenseFile.MatchString(f.Name()) {
			parts = append(parts, strings.TrimSpace(readFile(filepath.Join(dir, f.Name()))))
		}
	}
	return strings.Join(parts, "\n\n")
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	return string(data)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "notices:", err)
	os.Exit(1)
}
