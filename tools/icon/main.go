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
// launcher's icon: a rounded square with a soft top-to-bottom gradient of
// dark grey, and the mark in a light grey gradient over it.
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
		for a := 0; a < size; a++ {
			sum, count := 0.0, 0
			at := func(b int) int {
				if horizontal {
					return a*size + b
				}
				return b*size + a
			}
			for b := 0; b < r && b < size; b++ {
				sum += in[at(b)]
				count++
			}
			for b := 0; b < size; b++ {
				if b+r < size {
					sum += in[at(b+r)]
					count++
				}
				if b-r-1 >= 0 {
					sum -= in[at(b-r-1)]
					count--
				}
				out[at(b)] = sum / float64(count)
			}
		}
		return out
	}
	for i := 0; i < 2; i++ {
		f = pass(pass(f, true), false)
	}
	return f
}

// render dresses the master; bare gives only the mark in flat white on a
// transparent background (the site's header).
//
// The dressed mark reads as metal: its colour runs from light at the top to
// darker at the bottom, its edges are lit from the top left and shaded at
// the bottom right (a bevel from the coverage's slope), and it casts a soft
// shadow on the square.
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
	soft := blur(t, size, size/160)                // the bevel's slope
	shadow := blur(t, size, size/50)               // the drop shadow
	drop := int(math.Round(float64(size) * 0.014)) // its offset downwards
	lx, ly := -0.6, -0.8                           // light from the top left
	bevel := float64(size) / 24                    // how strongly the slope lights
	for y := 0; y < size; y++ {
		v := float64(y) / float64(size-1)
		for x := 0; x < size; x++ {
			i := y*size + x
			br, bg, bb := lerp(s.bgTop, s.bgBottom, v)
			if sy := y - drop; sy >= 0 {
				// The shadow darkens the square around and below the mark.
				k := 1 - 0.55*shadow[sy*size+x]*(1-t[i])
				br, bg, bb = br*k, bg*k, bb*k
			}
			fr, fg, fb := lerp(s.fgTop, s.fgBottom, v)
			if t[i] > 0 {
				gx, gy := 0.0, 0.0
				if x > 0 && x < size-1 {
					gx = soft[i+1] - soft[i-1]
				}
				if y > 0 && y < size-1 {
					gy = soft[i+size] - soft[i-size]
				}
				// Slopes facing the light brighten, those away from it darken.
				light := math.Max(-0.35, math.Min(0.6, (gx*lx+gy*ly)*bevel))
				fr, fg, fb = fr+(255-fr)*math.Max(0, light)+fr*math.Min(0, light), fg+(255-fg)*math.Max(0, light)+fg*math.Min(0, light), fb+(255-fb)*math.Max(0, light)+fb*math.Min(0, light)
			}
			a := squareAlpha(x, y, size, radius)
			c := t[i]
			out.SetNRGBA(x, y, color.NRGBA{
				R: uint8(math.Round(math.Min(255, br*(1-c)+fr*c))),
				G: uint8(math.Round(math.Min(255, bg*(1-c)+fg*c))),
				B: uint8(math.Round(math.Min(255, bb*(1-c)+fb*c))),
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
	bgTop := flag.String("bg-top", "3d3f40", "background gradient, top (rrggbb)")
	bgBottom := flag.String("bg-bottom", "1f2021", "background gradient, bottom (rrggbb)")
	fgTop := flag.String("fg-top", "f4f4f4", "mark gradient, top (rrggbb)")
	fgBottom := flag.String("fg-bottom", "b4b7b8", "mark gradient, bottom (rrggbb)")
	corner := flag.Float64("corner", 0.2, "corner radius as a fraction of the side")
	out := flag.String("out", "", "write only one PNG of -size here")
	size := flag.Int("size", 256, "size for -out")
	bare := flag.Bool("bare", false, "with -out: only the mark, flat white on a transparent background")
	flag.Parse()
	s := style{corner: *corner, bgTop: parse(*bgTop), bgBottom: parse(*bgBottom), fgTop: parse(*fgTop), fgBottom: parse(*fgBottom)}
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
