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

// A screenshot shared with the squad (squad-share.js): a small picture sent
// by itself when the player shares every screenshot, the larger one when
// they share one by hand or a squadmate asks for it. Its file's MD5 names
// it, so the same screenshot is never sent twice.

// squadShotSmall is the longest side of the picture shared by itself, and
// squadShotLarge that of the one shared in full: the relay carries 4 KB
// messages, so a small one is a dozen of them and a large one a hundred.
const (
	squadShotSmall = 640
	squadShotLarge = 1920
)

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
}

// SquadShot reads a screenshot of the screenshot folder (its name) for the
// squad: small, or large with full.
func (a *App) SquadShot(name string, full bool) (SquadShot, error) {
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
	side, quality := squadShotSmall, 70
	if full {
		side, quality = squadShotLarge, 80
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, fitWithin(img, side), &jpeg.Options{Quality: quality}); err != nil {
		return SquadShot{}, err
	}
	shot := SquadShot{Hash: hex.EncodeToString(sum[:]), Image: imaging.DataURL("image/jpeg", buffer.Bytes())}
	if _, err := position.ParseFilename(name); err == nil {
		shot.Positioned = true
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
	_, err = position.ParseFilename(name)
	shot := SquadShot{Hash: hex.EncodeToString(sum[:]), Positioned: err == nil}
	if a.screenshotIndex != nil {
		if r, ok := a.screenshotIndex.Get(name); ok {
			shot.Kind = r.Type
		}
	}
	return shot, nil
}

// fitWithin scales img down so its longest side is at most side.
func fitWithin(img image.Image, side int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= side && h <= side {
		return img
	}
	if w >= h {
		h, w = max(1, h*side/w), side
	} else {
		w, h = max(1, w*side/h), side
	}
	small := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), img, b, draw.Src, nil)
	return small
}
