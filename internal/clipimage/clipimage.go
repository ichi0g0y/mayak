// Package clipimage puts a picture on the system clipboard, as other apps
// paste it: a device-independent bitmap, and the PNG itself for the apps
// (browsers among them) that take PNG and keep its transparency.
package clipimage

import (
	"encoding/binary"
	"image"
	"image/draw"
)

// dib is img as a packed 32-bit device-independent bitmap: the
// BITMAPINFOHEADER, then the rows bottom-up in BGRA.
func dib(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	rgba := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	out := make([]byte, 40+w*h*4)
	header := out[:40]
	binary.LittleEndian.PutUint32(header[0:], 40)
	binary.LittleEndian.PutUint32(header[4:], uint32(w))
	binary.LittleEndian.PutUint32(header[8:], uint32(h))
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], 32)
	binary.LittleEndian.PutUint32(header[20:], uint32(w*h*4))
	pixels := out[40:]
	for y := 0; y < h; y++ {
		src := rgba.Pix[y*rgba.Stride : y*rgba.Stride+w*4]
		dst := pixels[(h-1-y)*w*4:]
		for x := 0; x < w; x++ {
			r, g, bl, a := src[x*4], src[x*4+1], src[x*4+2], src[x*4+3]
			dst[x*4], dst[x*4+1], dst[x*4+2], dst[x*4+3] = bl, g, r, a
		}
	}
	return out
}
