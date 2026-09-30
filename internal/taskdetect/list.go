package taskdetect

import (
	"image"
	"image/draw"

	"github.com/local/mayak/internal/screenscale"
)

// The task list on the left of a trader's Tasks screen, with "Show
// completed" ticked: each task a row whose background tells its state (a
// completed task's is blue, one in progress brown; a failed or not accepted
// one grey), under group headings (black). The selected row is lit and left
// out, as are rows the list cuts off.

// ListRow is a row of the list: its task's name, cut out for OCR, and its
// state by the row's colour.
type ListRow struct {
	Name  image.Image
	State string // "completed", "active" or "other"
}

// The list's place on the screen (2560 px wide): where its rows are, the
// strip at their right end whose colour is sampled (no name reaches it), and
// where a row's name is (past its icon; a row of a group is indented).
var (
	listRows   = image.Rect(10, 440, 745, 1360)
	listSample = image.Rect(650, 0, 740, 0)
	listName   = image.Rect(55, 0, 645, 0)
)

// The rows' colours (measured on 2560×1440 screenshots, the sampled strip's
// average) and how far one may be from them.
var rowColors = []struct {
	state   string
	r, g, b int
}{
	{"completed", 13, 23, 29},
	{"active", 32, 22, 9},
	{"other", 21, 22, 22},
}

const rowColorSlack = 12

// ListRows finds the full rows of the list on a Tasks screenshot. A row is
// a run of lines whose sampled colour stays the same, 70 to 100 px tall.
func ListRows(img image.Image) ([]ListRow, error) {
	sharp, err := screenscale.To1440p(img)
	if err != nil {
		return nil, err
	}
	smooth, err := screenscale.Smooth1440p(img)
	if err != nil {
		return nil, err
	}
	sample := func(y int) (int, int, int) {
		var r, g, b, n int
		for x := listSample.Min.X; x < listSample.Max.X; x++ {
			cr, cg, cb, _ := sharp.At(x, y).RGBA()
			r, g, b, n = r+int(cr>>8), g+int(cg>>8), b+int(cb>>8), n+1
		}
		return r / n, g / n, b / n
	}
	var out []ListRow
	start, sr, sg, sb := -1, 0, 0, 0
	flush := func(end int) {
		if start < 0 || end-start < 70 || end-start > 100 {
			return
		}
		for _, c := range rowColors {
			if abs(sr-c.r)+abs(sg-c.g)+abs(sb-c.b) <= rowColorSlack {
				name := image.Rect(listName.Min.X, start+4, listName.Max.X, end-4)
				out = append(out, ListRow{Name: cropImage(smooth, name), State: c.state})
				return
			}
		}
	}
	bottom := min(listRows.Max.Y, sharp.Bounds().Max.Y)
	for y := listRows.Min.Y; y < bottom; y++ {
		r, g, b := sample(y)
		if start >= 0 && abs(r-sr)+abs(g-sg)+abs(b-sb) < 18 {
			continue
		}
		flush(y)
		start, sr, sg, sb = y, r, g, b
	}
	// The last run reaches the list's end: cut off unless it ended above it.
	return out, nil
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func cropImage(img image.Image, r image.Rectangle) image.Image {
	r = r.Intersect(img.Bounds())
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Src)
	return dst
}
