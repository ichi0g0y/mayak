// Package imaging holds what the screenshot detectors and the OCR
// preprocessing share: fast pixel access, brightness, crops and data URLs.
// image.Image's At is an interface call plus a colour conversion per pixel;
// a 1440p scan makes millions of them, so the detectors read an RGBA
// buffer's bytes instead.
package imaging

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// Pixels is an image as 8-bit RGBA bytes with its origin at (0, 0), for
// reading pixels without an interface call. Make one with Of.
type Pixels struct{ *image.RGBA }

// Of gives img's pixels: img itself when it is already an *image.RGBA at
// the origin, else a copy (premultiplied, which is the same for the opaque
// screenshots this is used on).
func Of(img image.Image) Pixels {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Rect.Min == (image.Point{}) {
		return Pixels{rgba}
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return Pixels{dst}
}

// RGB is the pixel at x, y; the caller keeps x, y inside the image.
func (p Pixels) RGB(x, y int) (r, g, b uint8) {
	i := p.PixOffset(x, y)
	return p.Pix[i], p.Pix[i+1], p.Pix[i+2]
}

// Luma is the brightness of the pixel at x, y (see Luma).
func (p Pixels) Luma(x, y int) uint8 { return Luma(p.RGB(x, y)) }

// Gray is the pixel at x, y as color.GrayModel would convert it.
func (p Pixels) Gray(x, y int) uint8 { return Gray(p.RGB(x, y)) }

// Width and Height are the image's size.
func (p Pixels) Width() int  { return p.Rect.Dx() }
func (p Pixels) Height() int { return p.Rect.Dy() }

// Luma is the Rec. 601 brightness of a colour, 0–255: the detectors'
// measure of dark and bright.
func Luma(r, g, b uint8) uint8 {
	return uint8((299*uint32(r) + 587*uint32(g) + 114*uint32(b)) / 1000)
}

// LumaOf is Luma for a color.Color.
func LumaOf(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()
	return Luma(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// Gray converts as color.GrayModel does (the same weights and rounding, on
// the colour widened to 16 bits), for the OCR preprocessing, whose
// thresholds were tuned on it.
func Gray(r, g, b uint8) uint8 {
	r16, g16, b16 := uint32(r)*0x101, uint32(g)*0x101, uint32(b)*0x101
	return uint8((19595*r16 + 38470*g16 + 7471*b16 + 1<<15) >> 24)
}

// Crop copies the w×h area of src at x, y (in src's coordinates) into a
// new image at the origin.
func Crop(src image.Image, x, y, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), src, image.Point{X: x, Y: y}, draw.Src)
	return dst
}

// PNGDataURL encodes img as a PNG data URL.
func PNGDataURL(img image.Image) (string, error) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return "", err
	}
	return DataURL("image/png", buffer.Bytes()), nil
}

// DataURL is data as a base64 data URL of the given media type.
func DataURL(mediaType string, data []byte) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// Abs is |v|.
func Abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Clamp01 keeps v within 0 and 1.
func Clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
