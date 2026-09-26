// Command icon derives every icon from the master artwork tools/icon/mark.png
// (the hexagon and M as first drawn, khaki on near-black, 1024px) by
// recolouring it: build/appicon.png (the tray icon and the source of the
// macOS ICNS), build/windows/icon.ico (16 to 256px, PNG entries),
// frontend/public/favicon-32.png and favicon-256.png (the About logo), and
// the site's mark (site/public/assets/mayak-mark.png) plus its bare white
// version for the header (mayak-mark-white.png, transparent).
//
//	go run ./tools/icon -bg 2e2e2e -fg f2f2f2
//
// The master's two colours are read as the ends of a scale; every pixel's
// position on it (anti-aliased edges included) is kept and mapped onto the
// new pair, so the shape never changes.
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

// recolour maps the master onto bg and fg; bare leaves the background
// transparent, with the mark's coverage as alpha.
func recolour(master *image.NRGBA, bg, fg color.NRGBA, bare bool) *image.NRGBA {
	out := image.NewNRGBA(master.Bounds())
	for y := master.Bounds().Min.Y; y < master.Bounds().Max.Y; y++ {
		for x := master.Bounds().Min.X; x < master.Bounds().Max.X; x++ {
			t := coverage(master.NRGBAAt(x, y))
			if bare {
				out.SetNRGBA(x, y, color.NRGBA{fg.R, fg.G, fg.B, uint8(math.Round(255 * t))})
				continue
			}
			out.SetNRGBA(x, y, color.NRGBA{
				R: uint8(math.Round(float64(bg.R)*(1-t) + float64(fg.R)*t)),
				G: uint8(math.Round(float64(bg.G)*(1-t) + float64(fg.G)*t)),
				B: uint8(math.Round(float64(bg.B)*(1-t) + float64(fg.B)*t)),
				A: 255,
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
	bg := flag.String("bg", "2e2e2e", "background colour (rrggbb)")
	fg := flag.String("fg", "f2f2f2", "mark colour (rrggbb)")
	out := flag.String("out", "", "write only one PNG of -size here")
	size := flag.Int("size", 256, "size for -out")
	bare := flag.Bool("bare", false, "with -out: only the mark, on a transparent background")
	flag.Parse()
	b, f := parse(*bg), parse(*fg)
	master := loadMaster()
	if *out != "" {
		if err := os.WriteFile(*out, encodePNG(resize(recolour(master, b, f, *bare), *size)), 0o644); err != nil {
			log.Fatal(err)
		}
		return
	}
	full, white := recolour(master, b, f, false), recolour(master, b, f, true)
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
