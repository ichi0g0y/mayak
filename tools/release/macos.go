package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/image/draw"
)

// macOS gets a disk image with Mayak.app inside: the binary, an Info.plist,
// the icon and the notices, ad-hoc signed so Apple silicon runs it. The
// tar.gz with the bare binary is still built, for the in-app updater.

const bundleID = "com.ichi0g0y.mayak"

// buildApp lays out Mayak.app under dir and returns its path.
func buildApp(dir, bin, version, iconPNG string) (string, error) {
	app := filepath.Join(dir, "Mayak.app")
	contents := filepath.Join(app, "Contents")
	for _, sub := range []string{"MacOS", "Resources"} {
		if err := os.MkdirAll(filepath.Join(contents, sub), 0o755); err != nil {
			return "", err
		}
	}
	if err := copyPath(filepath.Join(bin, "Mayak"), filepath.Join(contents, "MacOS", "Mayak"), 0o755); err != nil {
		return "", err
	}
	if err := copyPath(filepath.Join(bin, "THIRD_PARTY_NOTICES.txt"), filepath.Join(contents, "Resources", "THIRD_PARTY_NOTICES.txt"), 0o644); err != nil {
		return "", err
	}
	icns, err := makeICNS(iconPNG)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(contents, "Resources", "icon.icns"), icns, 0o644); err != nil {
		return "", err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key><string>en</string>
	<key>CFBundleDisplayName</key><string>MAYAK</string>
	<key>CFBundleExecutable</key><string>Mayak</string>
	<key>CFBundleIconFile</key><string>icon.icns</string>
	<key>CFBundleIdentifier</key><string>%s</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>CFBundleName</key><string>MAYAK</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>%s</string>
	<key>CFBundleVersion</key><string>%s</string>
	<key>LSMinimumSystemVersion</key><string>11.0</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>NSHumanReadableCopyright</key><string>Copyright MAYAK contributors. GPL-3.0.</string>
</dict>
</plist>
`, bundleID, version, version)
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o644); err != nil {
		return "", err
	}
	// An ad-hoc signature: Apple silicon refuses unsigned code outright.
	if codesign, err := exec.LookPath("codesign"); err == nil {
		cmd := exec.Command(codesign, "--force", "--deep", "--sign", "-", app)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("codesign: %w", err)
		}
	}
	return app, nil
}

// buildDMG packs the folder holding Mayak.app (and an Applications link)
// into a compressed disk image with hdiutil.
func buildDMG(staging, target string) error {
	if err := os.Symlink("/Applications", filepath.Join(staging, "Applications")); err != nil && !os.IsExist(err) {
		return err
	}
	os.Remove(target)
	cmd := exec.Command("hdiutil", "create", "-volname", "MAYAK", "-srcfolder", staging, "-ov", "-format", "UDZO", target)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hdiutil: %w", err)
	}
	return nil
}

func copyPath(source, target string, perm os.FileMode) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, perm)
}

// makeICNS renders the PNG icon at the sizes macOS wants and packs them
// into an ICNS container (PNG-encoded entries, as modern macOS reads).
func makeICNS(iconPNG string) ([]byte, error) {
	file, err := os.Open(iconPNG)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	source, err := png.Decode(file)
	if err != nil {
		return nil, err
	}
	entries := []struct {
		kind string
		size int
	}{{"icp4", 16}, {"icp5", 32}, {"icp6", 64}, {"ic07", 128}, {"ic08", 256}, {"ic09", 512}, {"ic10", 1024}}
	var body bytes.Buffer
	for _, entry := range entries {
		scaled := image.NewNRGBA(image.Rect(0, 0, entry.size, entry.size))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), source, source.Bounds(), draw.Over, nil)
		var data bytes.Buffer
		if err := png.Encode(&data, scaled); err != nil {
			return nil, err
		}
		body.WriteString(entry.kind)
		binary.Write(&body, binary.BigEndian, uint32(8+data.Len()))
		body.Write(data.Bytes())
	}
	var out bytes.Buffer
	out.WriteString("icns")
	binary.Write(&out, binary.BigEndian, uint32(8+body.Len()))
	out.Write(body.Bytes())
	return out.Bytes(), nil
}
