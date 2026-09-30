package app

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"

	"github.com/local/mayak/internal/imaging"
	"github.com/local/mayak/internal/position"
)

// A screenshot shared with the squad (squad-share.js): a small picture on
// a position (the bubble by its arrow on the map), the larger one when the
// player shares one by hand or a squadmate asks for it. Its file's MD5
// names it, so the same screenshot is never sent twice.

// squadShotSmall is the longest side of the small picture: the relay
// carries 4 KB messages, so it is a dozen of them (a large one, a hundred).
const squadShotSmall = 640

// squadShotBoxes are the sizes a large picture is made at, a box it fits
// in ("full": as taken).
var squadShotBoxes = map[string][2]int{"1080": {1920, 1080}, "720": {1280, 720}}

// SquadShot is a screenshot made ready for the squad.
type SquadShot struct {
	// Hash is the file's MD5 (hex).
	Hash string `json:"hash"`
	// Image is the picture, a JPEG data URL.
	Image string `json:"image"`
	// Positioned tells a screenshot with a position in its name (taken in a
	// raid with the position shown), and Map the map it was taken on.
	Positioned bool   `json:"positioned"`
	Map        string `json:"map,omitempty"`
	// Kind is what MAYAK recognized it as (tasks, item, position, profile,
	// unknown; "" before its analysis).
	Kind string `json:"kind,omitempty"`
	// X and Z are the position in its name (with Positioned).
	X float64 `json:"x,omitempty"`
	Z float64 `json:"z,omitempty"`
}

// SquadShot reads a screenshot of the screenshot folder (its name) for the
// squad at a size: "small", "720", "1080" or "full" (as taken).
func (a *App) SquadShot(name, size string) (SquadShot, error) {
	dir := a.screenshotDir()
	if dir == "" || name != filepath.Base(name) || !screenshotName(name) {
		return SquadShot{}, errors.New("invalid screenshot")
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return SquadShot{}, err
	}
	sum := md5.Sum(data)
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return SquadShot{}, err
	}
	fitted, quality := fitWithin(img, squadShotSmall, squadShotSmall), 70
	if size != "small" {
		fitted, quality = img, 80
		if box, ok := squadShotBoxes[size]; ok {
			fitted = fitWithin(img, box[0], box[1])
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, fitted, &jpeg.Options{Quality: quality}); err != nil {
		return SquadShot{}, err
	}
	shot := SquadShot{Hash: hex.EncodeToString(sum[:]), Image: imaging.DataURL("image/jpeg", buffer.Bytes())}
	if p, err := position.ParseFilename(name); err == nil {
		shot.Positioned, shot.X, shot.Z = true, p.X, p.Z
	}
	if a.screenshotIndex != nil {
		if r, ok := a.screenshotIndex.Get(name); ok {
			shot.Kind, shot.Map = r.Type, r.Map
		}
	}
	if shot.Map == "" && shot.Positioned {
		a.mu.RLock()
		shot.Map = a.status.CurrentMap
		a.mu.RUnlock()
	}
	return shot, nil
}

// SquadShotHash tells a screenshot's MD5 and whether it has a position,
// without making its picture: to know whether it goes before making it.
func (a *App) SquadShotHash(name string) (SquadShot, error) {
	dir := a.screenshotDir()
	if dir == "" || name != filepath.Base(name) || !screenshotName(name) {
		return SquadShot{}, errors.New("invalid screenshot")
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return SquadShot{}, err
	}
	sum := md5.Sum(data)
	shot := SquadShot{Hash: hex.EncodeToString(sum[:])}
	if p, err := position.ParseFilename(name); err == nil {
		shot.Positioned, shot.X, shot.Z = true, p.X, p.Z
	}
	if a.screenshotIndex != nil {
		if r, ok := a.screenshotIndex.Get(name); ok {
			shot.Kind = r.Type
		}
	}
	return shot, nil
}

// fitWithin scales img down to fit in a box of width by height.
func fitWithin(img image.Image, width, height int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= width && h <= height {
		return img
	}
	if w*height >= h*width {
		h, w = max(1, h*width/w), width
	} else {
		w, h = max(1, w*height/h), height
	}
	small := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), img, b, draw.Src, nil)
	return small
}
