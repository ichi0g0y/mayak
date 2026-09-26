// Command icon derives every icon from the master artwork tools/icon/mark.png
// (the hexagon and M as first drawn, khaki on near-black, 1024px):
// build/appicon.png (the tray icon and the source of the macOS ICNS),
// build/windows/icon.ico (16 to 256px, PNG entries), frontend/public/
// favicon-32.png and favicon-256.png (the About logo), and the site's mark
// (site/public/assets/mayak-mark.png) plus its bare white version for the
// header (mayak-mark-white.png, transparent).
//
//	go run ./tools/icon
//
// The master's two colours are read as the ends of a scale; every pixel's
// position on it (anti-aliased edges included) is kept, so the shape never
// changes. What changes is the dressing, in the manner of the official
// launcher's icon: a rounded square in a cool dark grey gradient, the mark in
// a grey gradient with a light haze around it, grain and faint scanlines.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"

	"golang.org/x/image/draw"
)

// The master's colours.
var masterBG = color.NRGBA{0x1b, 0x1c, 0x1a, 255}
var masterFG = color.NRGBA{0xcd, 0xc6, 0xae, 255}

// coverage is how far a master pixel sits from the background towards the
// mark, 0 to 1.
func coverage(c color.NRGBA) float64 {
	dr, dg, db := float64(masterFG.R)-float64(masterBG.R), float64(masterFG.G)-float64(masterBG.G), float64(masterFG.B)-float64(masterBG.B)
	pr, pg, pb := float64(c.R)-float64(masterBG.R), float64(c.G)-float64(masterBG.G), float64(c.B)-float64(masterBG.B)
	t := (pr*dr + pg*dg + pb*db) / (dr*dr + dg*dg + db*db)
	return math.Max(0, math.Min(1, t))
}

func loadMaster() *image.NRGBA {
	f, err := os.Open("tools/icon/mark.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	out := image.NewNRGBA(src.Bounds())
	draw.Draw(out, out.Bounds(), src, src.Bounds().Min, draw.Src)
	return out
}

// style is the icon's dressing: the square's corner radius as a fraction of
// its side, and the top and bottom of the background and mark gradients.
type style struct {
	corner                           float64
	bgTop, bgBottom, fgTop, fgBottom color.NRGBA
	haze                             float64     // strength of the light haze around the mark
	hazeColor                        color.NRGBA // its colour
	grain                            float64     // amplitude of the grain, in levels
	scan                             float64     // depth of the scanlines, in levels
}

func lerp(a, b color.NRGBA, t float64) (float64, float64, float64) {
	return float64(a.R)*(1-t) + float64(b.R)*t, float64(a.G)*(1-t) + float64(b.G)*t, float64(a.B)*(1-t) + float64(b.B)*t
}

// squareAlpha is the anti-aliased coverage of a rounded square of the given
// side and corner radius at pixel (x, y).
func squareAlpha(x, y, size int, radius float64) float64 {
	half := float64(size) / 2
	px, py := float64(x)+0.5-half, float64(y)+0.5-half
	dx, dy := math.Max(math.Abs(px)-(half-radius), 0), math.Max(math.Abs(py)-(half-radius), 0)
	d := math.Hypot(dx, dy) - radius
	return math.Max(0, math.Min(1, 0.5-d))
}

// field is the master's coverage as a float image, for the shading passes.
func field(master *image.NRGBA) ([]float64, int) {
	size := master.Bounds().Dx()
	f := make([]float64, size*size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			f[y*size+x] = coverage(master.NRGBAAt(master.Bounds().Min.X+x, master.Bounds().Min.Y+y))
		}
	}
	return f, size
}

// blur is a separable box blur of radius r, applied twice for a soft falloff.
func blur(f []float64, size, r int) []float64 {
	if r < 1 {
		return f
	}
	pass := func(in []float64, horizontal bool) []float64 {
		out := make([]float64, len(in))
		at := func(a, b int) int {
			if horizontal {
				return a*size + b
			}
			return b*size + a
		}
		for a := 0; a < size; a++ {
			sum, count := 0.0, 0
			for b := 0; b < r && b < size; b++ {
				sum += in[at(a, b)]
				count++
			}
			for b := 0; b < size; b++ {
				if b+r < size {
					sum += in[at(a, b+r)]
					count++
				}
				if b-r-1 >= 0 {
					sum -= in[at(a, b-r-1)]
					count--
				}
				out[at(a, b)] = sum / float64(count)
			}
		}
		return out
	}
	for i := 0; i < 2; i++ {
		f = pass(pass(f, true), false)
	}
	return f
}

// grain is a deterministic noise value in [-1, 1) for a pixel, so builds
// are reproducible.
func grain(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h&0xffff)/32768 - 1
}

// render dresses the master; bare gives only the mark in flat white on a
// transparent background (the site's header).
//
// The dressing follows the launcher's icon: a rounded square in a cool dark
// grey gradient, the mark in a mid-grey gradient (light at the top, darker
// at the bottom) with a soft light haze around it on the square, and a fine
// grain over everything, so both read as brushed metal rather than flat
// paint. No bevel: the mark stays flush.
func render(master *image.NRGBA, s style, bare bool) *image.NRGBA {
	t, size := field(master)
	radius := float64(size) * s.corner
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	if bare {
		for i, v := range t {
			out.SetNRGBA(i%size, i/size, color.NRGBA{0xff, 0xff, 0xff, uint8(math.Round(255 * v))})
		}
		return out
	}
	haze := blur(t, size, size/36)
	for y := 0; y < size; y++ {
		v := float64(y) / float64(size-1)
		for x := 0; x < size; x++ {
			i := y*size + x
			u := float64(x) / float64(size-1)
			br, bg, bb := lerp(s.bgTop, s.bgBottom, math.Max(0, math.Min(1, 0.85*v+0.15*u)))
			// The haze lightens the square around the mark.
			if h := haze[i] * s.haze; h > 0 {
				br, bg, bb = br+(float64(s.hazeColor.R)-br)*h, bg+(float64(s.hazeColor.G)-bg)*h, bb+(float64(s.hazeColor.B)-bb)*h
			}
			fr, fg, fb := lerp(s.fgTop, s.fgBottom, math.Max(0, math.Min(1, 0.9*v+0.1*u)))
			cov := t[i]
			r, g, b := br*(1-cov)+fr*cov, bg*(1-cov)+fg*cov, bb*(1-cov)+fb*cov
			// Grain, a little stronger on the mark, and faint scanlines: every
			// third row at the 256px scale is darker and the next a shade brighter, like a screen.
			g0 := grain(x, y) * s.grain * (1 + 0.6*cov)
			switch (y / max(1, size/256)) % 3 {
			case 1:
				g0 -= s.scan // the dark line
			case 2:
				g0 += s.scan * 0.35 // and the faint bright line under it, like a screen's
			}
			r, g, b = r+g0, g+g0, b+g0
			a := squareAlpha(x, y, size, radius)
			out.SetNRGBA(x, y, color.NRGBA{
				R: uint8(math.Round(math.Max(0, math.Min(255, r)))),
				G: uint8(math.Round(math.Max(0, math.Min(255, g)))),
				B: uint8(math.Round(math.Max(0, math.Min(255, b)))),
				A: uint8(math.Round(255 * a)),
			})
		}
	}
	return out
}

func resize(src *image.NRGBA, size int) *image.NRGBA {
	if src.Bounds().Dx() == size {
		return src
	}
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(out, out.Bounds(), src, src.Bounds(), draw.Over, nil)
	return out
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

// ico writes a Windows icon whose entries are PNG images.
func ico(full *image.NRGBA, sizes []int) []byte {
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	var images [][]byte
	for _, size := range sizes {
		images = append(images, encodePNG(resize(full, size)))
	}
	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		dim := uint8(size)
		if size >= 256 {
			dim = 0
		}
		out.Write([]byte{dim, dim, 0, 0})
		binary.Write(&out, binary.LittleEndian, [2]uint16{1, 32})
		binary.Write(&out, binary.LittleEndian, [2]uint32{uint32(len(images[i])), uint32(offset)})
		offset += len(images[i])
	}
	for _, data := range images {
		out.Write(data)
	}
	return out.Bytes()
}

func parse(hex string) color.NRGBA {
	var c color.NRGBA
	if _, err := fmt.Sscanf(hex, "%02x%02x%02x", &c.R, &c.G, &c.B); err != nil {
		log.Fatalf("colour %q: want rrggbb", hex)
	}
	c.A = 255
	return c
}

func main() {
	bgTop := flag.String("bg-top", "33383a", "background gradient, top (rrggbb)")
	bgBottom := flag.String("bg-bottom", "24272a", "background gradient, bottom (rrggbb)")
	fgTop := flag.String("fg-top", "d2d2d2", "mark gradient, top (rrggbb)")
	fgBottom := flag.String("fg-bottom", "7c7e80", "mark gradient, bottom (rrggbb)")
	haze := flag.Float64("haze", 0.5, "strength of the light haze around the mark, 0 to 1")
	hazeColor := flag.String("haze-color", "9a9ea0", "colour of the haze (rrggbb)")
	grainLevels := flag.Float64("grain", 5, "amplitude of the grain, in levels of 255")
	scan := flag.Float64("scanlines", 10, "depth of the scanlines, in levels of 255")
	corner := flag.Float64("corner", 0.12, "corner radius as a fraction of the side (the launcher's is about 0.12)")
	out := flag.String("out", "", "write only one PNG of -size here")
	size := flag.Int("size", 256, "size for -out")
	bare := flag.Bool("bare", false, "with -out: only the mark, flat white on a transparent background")
	flag.Parse()
	s := style{corner: *corner, bgTop: parse(*bgTop), bgBottom: parse(*bgBottom), fgTop: parse(*fgTop), fgBottom: parse(*fgBottom), haze: *haze, hazeColor: parse(*hazeColor), grain: *grainLevels, scan: *scan}
	master := loadMaster()
	if *out != "" {
		if err := os.WriteFile(*out, encodePNG(resize(render(master, s, *bare), *size)), 0o644); err != nil {
			log.Fatal(err)
		}
		return
	}
	full, white := render(master, s, false), render(master, s, true)
	must := func(err error) {
		if err != nil {
			log.Fatal(err)
		}
	}
	must(os.WriteFile("build/appicon.png", encodePNG(full), 0o644))
	must(os.WriteFile("build/windows/icon.ico", ico(full, []int{16, 24, 32, 48, 64, 128, 256}), 0o644))
	must(os.WriteFile("frontend/public/favicon-32.png", encodePNG(resize(full, 32)), 0o644))
	must(os.WriteFile("frontend/public/favicon-256.png", encodePNG(resize(full, 256)), 0o644))
	must(os.WriteFile("site/public/assets/mayak-mark.png", encodePNG(full), 0o644))
	must(os.WriteFile("site/public/assets/mayak-mark-white.png", encodePNG(resize(white, 512)), 0o644))
	fmt.Println("wrote build/appicon.png, build/windows/icon.ico, frontend/public/favicon-32.png, favicon-256.png, site/public/assets/mayak-mark.png, mayak-mark-white.png")
}
