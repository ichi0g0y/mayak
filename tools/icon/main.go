// Command icon renders the MAYAK mark (the hexagon and M of
// frontend/public/favicon.svg) into the raster icons the builds embed:
// build/appicon.png (1024px, also the tray icon and the source of the macOS
// bundle's ICNS), build/windows/icon.ico (16 to 256px, PNG entries) and
// frontend/public/favicon-32.png, and rewrites the SVG's colours to match.
//
//	go run ./tools/icon -bg 2f3232 -fg cfd2d1
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
	"regexp"

	"golang.org/x/image/vector"
)

// The geometry, in the SVG's 32-unit box: a hexagon ring (circumradius
// 13.4 about (16, 16.5), 2.6 wide) around an M clipped to the hexagon of
// circumradius 10.5.
const (
	box       = 32.0
	cx, cy    = 16.0, 16.5
	ringR     = 13.4
	ringWidth = 2.6
	clipR     = 10.5
	cornerR   = 6.0
)

var leftM = [][2]float64{{8.56, 8.30}, {12.13, 8.30}, {15.08, 13.93}, {15.08, 19.12}, {12.66, 16.86}, {12.66, 24.30}, {8.56, 24.30}}

func hexagon(r float64) [][2]float64 {
	var points [][2]float64
	for i := 0; i < 6; i++ {
		a := math.Pi/6 + float64(i)*math.Pi/3 // flat sides left and right, points top and bottom
		points = append(points, [2]float64{cx + r*math.Cos(a-math.Pi/2), cy + r*math.Sin(a-math.Pi/2)})
	}
	return points
}

func mirror(points [][2]float64) [][2]float64 {
	out := make([][2]float64, len(points))
	for i, p := range points {
		out[i] = [2]float64{box - p[0], p[1]}
	}
	return out
}

// mask rasterises polygons into a coverage image of the given size.
func mask(size int, polygons ...[][2]float64) *image.Alpha {
	scale := float64(size) / box
	r := vector.NewRasterizer(size, size)
	for _, poly := range polygons {
		for i, p := range poly {
			x, y := float32(p[0]*scale), float32(p[1]*scale)
			if i == 0 {
				r.MoveTo(x, y)
			} else {
				r.LineTo(x, y)
			}
		}
		r.ClosePath()
	}
	out := image.NewAlpha(image.Rect(0, 0, size, size))
	r.Draw(out, out.Bounds(), image.Opaque, image.Point{})
	return out
}

// roundedRect rasterises the background square with rounded corners.
func roundedRect(size int) *image.Alpha {
	scale := float64(size) / box
	s, rad := float32(size), float32(cornerR*scale)
	const k = 0.5523 // cubic approximation of a quarter circle
	r := vector.NewRasterizer(size, size)
	r.MoveTo(rad, 0)
	r.LineTo(s-rad, 0)
	r.CubeTo(s-rad+rad*k, 0, s, rad-rad*k, s, rad)
	r.LineTo(s, s-rad)
	r.CubeTo(s, s-rad+rad*k, s-rad+rad*k, s, s-rad, s)
	r.LineTo(rad, s)
	r.CubeTo(rad-rad*k, s, 0, s-rad+rad*k, 0, s-rad)
	r.LineTo(0, rad)
	r.CubeTo(0, rad-rad*k, rad-rad*k, 0, rad, 0)
	r.ClosePath()
	out := image.NewAlpha(image.Rect(0, 0, size, size))
	r.Draw(out, out.Bounds(), image.Opaque, image.Point{})
	return out
}

func render(size int, bg, fg color.NRGBA) *image.NRGBA {
	base := roundedRect(size)
	outer, inner := mask(size, hexagon(ringR+ringWidth/2)), mask(size, hexagon(ringR-ringWidth/2))
	letter, clip := mask(size, leftM, mirror(leftM)), mask(size, hexagon(clipR))
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			i := y*size + x
			a := float64(base.Pix[i]) / 255
			ring := float64(outer.Pix[i]) / 255 * (1 - float64(inner.Pix[i])/255)
			m := float64(letter.Pix[i]) / 255 * float64(clip.Pix[i]) / 255
			f := math.Min(1, ring+m)
			// The mark over the background, both within the rounded square.
			c := color.NRGBA{
				R: uint8(math.Round(float64(bg.R)*(1-f) + float64(fg.R)*f)),
				G: uint8(math.Round(float64(bg.G)*(1-f) + float64(fg.G)*f)),
				B: uint8(math.Round(float64(bg.B)*(1-f) + float64(fg.B)*f)),
				A: uint8(math.Round(255 * a)),
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

// ico writes a Windows icon whose entries are PNG images.
func ico(sizes []int, bg, fg color.NRGBA) []byte {
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	var images [][]byte
	for _, size := range sizes {
		images = append(images, encodePNG(render(size, bg, fg)))
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
	bg := flag.String("bg", "2f3232", "background colour (rrggbb)")
	fg := flag.String("fg", "cfd2d1", "mark colour (rrggbb)")
	out := flag.String("out", "", "write only a preview PNG of this size's rendering here (with -size)")
	size := flag.Int("size", 48, "preview size")
	flag.Parse()
	b, f := parse(*bg), parse(*fg)
	if *out != "" {
		if err := os.WriteFile(*out, encodePNG(render(*size, b, f)), 0o644); err != nil {
			log.Fatal(err)
		}
		return
	}
	must := func(err error) {
		if err != nil {
			log.Fatal(err)
		}
	}
	must(os.WriteFile("build/appicon.png", encodePNG(render(1024, b, f)), 0o644))
	must(os.WriteFile("frontend/public/favicon-32.png", encodePNG(render(32, b, f)), 0o644))
	must(os.WriteFile("build/windows/icon.ico", ico([]int{16, 24, 32, 48, 64, 128, 256}, b, f), 0o644))
	svg, err := os.ReadFile("frontend/public/favicon.svg")
	must(err)
	svg = regexp.MustCompile(`fill="#[0-9a-fA-F]{6}"`).ReplaceAllFunc(svg, func(m []byte) []byte {
		if bytes.Contains(m, []byte("1b1c1a")) || bytes.Contains(m, []byte(*bg)) {
			return []byte(`fill="#` + *bg + `"`)
		}
		return []byte(`fill="#` + *fg + `"`)
	})
	svg = regexp.MustCompile(`stroke="#[0-9a-fA-F]{6}"`).ReplaceAll(svg, []byte(`stroke="#`+*fg+`"`))
	must(os.WriteFile("frontend/public/favicon.svg", svg, 0o644))
	fmt.Println("wrote build/appicon.png, build/windows/icon.ico, frontend/public/favicon-32.png, frontend/public/favicon.svg")
}
