// Command ocrtrain generates Tesseract LSTM training data for Escape from
// Tarkov titles: every English quest and item name from a MAYAK catalog is
// drawn in the game's font (Bender, SIL OFL) the way the game shows it, then
// passed through MAYAK' own Tesseract preprocessing.
//
//	ocrtrain -catalog %AppData%\Mayak\catalog\pve.json -fonts <dir with Bender*.otf> -out <dir>
//
// With -lang ja it draws the Japanese names instead, in fonts like the ones
// the game uses for Japanese (it embeds Noto Sans CJK and falls back to
// Meiryo): e.g. meiryo.ttc and YuGothR/M.ttc in -fonts, and Bender in
// -latin-fonts for the Latin letters and digits the names mix in. (A variable
// font draws at its default, often thinnest, weight: use static fonts.)
//
// It writes <out>/lines/*.png with matching .gt.txt and .box files, and
// train.txt / eval.txt listing the .lstmf files Tesseract will produce from
// them (see docs in tools/ocrtrain/README.md for the full training run).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/local/mayak/internal/locale"
	"github.com/local/mayak/internal/ocr"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func main() {
	catalog := flag.String("catalog", "", "MAYAK catalog snapshot (pve.json, regular.json, ...)")
	fonts := flag.String("fonts", "", "directory containing the fonts (.otf, .ttf, .ttc)")
	lang := flag.String("lang", "en", "names to draw: en (English), or another language's code (ja)")
	latinFonts := flag.String("latin-fonts", "", "with another -lang: fonts for the Latin letters and digits mixed into its names (the game draws them in Bender)")
	out := flag.String("out", "", "output directory")
	variants := flag.Int("variants", 2, "renderings per name")
	evalShare := flag.Int("eval-percent", 5, "share of names held out for evaluation")
	flag.Parse()
	if *catalog == "" || *fonts == "" || *out == "" {
		log.Fatal("-catalog, -fonts and -out are required")
	}
	names, err := catalogNames(*catalog, *lang)
	if err != nil {
		log.Fatal(err)
	}
	faces, err := loadFonts(*fonts)
	if err != nil {
		log.Fatal(err)
	}
	// Without Latin fonts, the name's own font draws its Latin letters too.
	var latin []*opentype.Font
	if *latinFonts != "" {
		if latin, err = loadFonts(*latinFonts); err != nil {
			log.Fatal(err)
		}
	}
	lines := filepath.Join(*out, "lines")
	if err := os.MkdirAll(lines, 0o755); err != nil {
		log.Fatal(err)
	}
	var train, eval []string
	for i, name := range names {
		held := hash(name)%100 < uint32(*evalShare)
		for v := 0; v < *variants; v++ {
			rng := rand.New(rand.NewSource(int64(hash(name)) + int64(v)))
			base := filepath.Join(lines, fmt.Sprintf("%05d_%d", i, v))
			f := faces[rng.Intn(len(faces))]
			latinFont := f
			if len(latin) > 0 {
				latinFont = latin[rng.Intn(len(latin))]
			}
			img := render(name, f, latinFont, rng)
			if err := writeSample(base, ocr.PrepareForTesseract(img), name); err != nil {
				log.Fatal(err)
			}
			if held {
				eval = append(eval, base+".lstmf")
			} else {
				train = append(train, base+".lstmf")
			}
		}
	}
	rand.New(rand.NewSource(1)).Shuffle(len(train), func(i, j int) { train[i], train[j] = train[j], train[i] })
	must(os.WriteFile(filepath.Join(*out, "train.txt"), []byte(strings.Join(train, "\n")+"\n"), 0o644))
	must(os.WriteFile(filepath.Join(*out, "eval.txt"), []byte(strings.Join(eval, "\n")+"\n"), 0o644))
	fmt.Printf("%d names, %d training and %d evaluation lines\n", len(names), len(train), len(eval))
}

// catalogNames returns the quest names and item names and short names in
// lang. English skips lines with characters the English model cannot
// represent; Japanese names often mix in Latin words and digits.
func catalogNames(path, lang string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snapshot struct {
		Resources struct {
			Tasks struct {
				Data struct {
					Tasks map[string]struct{ Name string } `json:"tasks"`
				} `json:"data"`
			} `json:"tasks"`
			TasksEN struct {
				Data map[string]string `json:"data"`
			} `json:"tasks_en"`
			Items struct {
				Data struct {
					Items map[string]struct{ Name, ShortName string } `json:"items"`
				} `json:"data"`
			} `json:"items"`
			ItemsEN struct {
				Data map[string]string `json:"data"`
			} `json:"items_en"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	r := snapshot.Resources
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.Join(strings.Fields(name), " ")
		// Characters, not bytes: a Japanese character takes three bytes.
		if name == "" || utf8.RuneCountInString(name) > 64 || seen[name] {
			return
		}
		for _, c := range name {
			if !unicode.IsPrint(c) || lang == "en" && c > unicode.MaxASCII {
				return
			}
		}
		seen[name] = true
	}
	tasks, items := r.TasksEN.Data, r.ItemsEN.Data
	if lang != "en" {
		// The names in another language (see internal/locale): items_ja, ...
		var other struct {
			Resources map[string]json.RawMessage `json:"resources"`
		}
		if err := json.Unmarshal(data, &other); err != nil {
			return nil, err
		}
		names := func(resource string) map[string]string {
			var r struct {
				Data map[string]string `json:"data"`
			}
			_ = json.Unmarshal(other.Resources[resource], &r)
			return r.Data
		}
		tasks, items = names(locale.Resource("tasks", lang)), names(locale.Resource("items", lang))
		if len(items) == 0 {
			return nil, fmt.Errorf("%s has no names in %s (items_%s)", path, lang, lang)
		}
	}
	for _, t := range r.Tasks.Data.Tasks {
		add(tasks[t.Name])
	}
	for _, it := range r.Items.Data.Items {
		add(items[it.Name])
		add(items[it.ShortName])
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func loadFonts(dir string) ([]*opentype.Font, error) {
	var files []string
	for _, pattern := range []string{"*.otf", "*.ttf", "*.ttc"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	var faces []*opentype.Font
	for _, file := range files {
		// The game draws titles upright and thin: mostly light and regular
		// weights (listed twice to be picked more often), sometimes bold.
		lower := strings.ToLower(filepath.Base(file))
		if strings.Contains(lower, "italic") || strings.Contains(lower, "black") {
			continue
		}
		copies := 2
		if strings.Contains(lower, "bold") {
			copies = 1
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		// A collection (.ttc) holds several faces; its upright ones are used.
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		for i := range collection.NumFonts() {
			f, err := collection.Font(i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", file, err)
			}
			if name, _ := f.Name(nil, sfnt.NameIDFull); strings.Contains(strings.ToLower(name), "italic") {
				continue
			}
			for range copies {
				faces = append(faces, f)
			}
		}
	}
	if len(faces) == 0 {
		return nil, fmt.Errorf("no usable fonts in %s", dir)
	}
	return faces, nil
}

// render draws name like a game title crop: a light panel with dark text and
// sometimes a quest icon behind a separator line (character tasks), or a dark
// panel with light text (trader tasks, item names), with some size, position
// and noise variation.
//
// ASCII letters, digits and punctuation are drawn with latin (Bender in the
// game, also within Japanese names), the rest with f.
func render(name string, f, latin *opentype.Font, rng *rand.Rand) image.Image {
	size := 19 + rng.Float64()*9
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		log.Fatal(err)
	}
	defer face.Close()
	latinFace := face
	if latin != f {
		if latinFace, err = opentype.NewFace(latin, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone}); err != nil {
			log.Fatal(err)
		}
		defer latinFace.Close()
	}
	faceFor := func(r rune) font.Face {
		if r <= unicode.MaxASCII {
			return latinFace
		}
		return face
	}
	light := rng.Intn(2) == 0
	var bg, fg uint8
	if light {
		bg, fg = uint8(125+rng.Intn(40)), uint8(15+rng.Intn(35))
	} else {
		bg, fg = uint8(12+rng.Intn(25)), uint8(200+rng.Intn(50))
	}
	// The game spaces letters a little wider than the font does.
	tracking := rng.Intn(3)
	textWidth := tracking * utf8.RuneCountInString(name)
	for _, r := range name {
		textWidth += font.MeasureString(faceFor(r), string(r)).Ceil()
	}
	left := 12 + rng.Intn(10)
	if light && rng.Intn(2) == 0 {
		left = 70 + rng.Intn(10) // room for an icon and the separator
	}
	h := int(size*2.2) + rng.Intn(20)
	img := image.NewRGBA(image.Rect(0, 0, left+textWidth+20+rng.Intn(60), h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.Gray{Y: bg}}, image.Point{}, draw.Src)
	if left >= 70 {
		for y := 0; y < h; y++ {
			img.Set(left-26, y, color.Gray{Y: 18})
			img.Set(left-25, y, color.Gray{Y: 18})
		}
		icon := image.Rect(8, h/2-10, 32, h/2+10)
		draw.Draw(img, icon, &image.Uniform{color.Gray{Y: uint8(200 + rng.Intn(55))}}, image.Point{}, draw.Src)
	}
	baseline := h/2 + int(size*0.35) + rng.Intn(5) - 2
	d := font.Drawer{Dst: img, Src: &image.Uniform{color.Gray{Y: fg}}, Face: face, Dot: fixed.P(left, baseline)}
	for _, r := range name {
		d.Face = faceFor(r)
		d.DrawString(string(r))
		d.Dot.X += fixed.I(tracking)
	}
	// Screenshot-like softness and sensor-free noise.
	for i := range img.Pix {
		if i%4 == 3 {
			continue
		}
		v := int(img.Pix[i]) + rng.Intn(9) - 4
		img.Pix[i] = uint8(max(0, min(255, v)))
	}
	return img
}

// writeSample writes base.png, base.gt.txt and a line box file for text: one
// box per character spanning the whole line, which is what Tesseract's LSTM
// training expects (tesstrain's generate_line_box.py writes the same).
func writeSample(base string, img image.Image, text string) error {
	f, err := os.Create(base + ".png")
	if err != nil {
		return err
	}
	if err = png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.WriteFile(base+".gt.txt", []byte(text+"\n"), 0o644); err != nil {
		return err
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var box strings.Builder
	for _, c := range text {
		fmt.Fprintf(&box, "%c 0 0 %d %d 0\n", c, w, h)
	}
	fmt.Fprintf(&box, "\t 0 0 %d %d 0\n", w, h)
	return os.WriteFile(base+".box", []byte(box.String()), 0o644)
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
