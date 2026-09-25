package position

import (
	"errors"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/local/mayak/internal/model"
)

var filenamePattern = regexp.MustCompile(`(?i)^\d{4}-\d{2}-\d{2}\[\d{2}-\d{2}\]_?(-?\d+\.\d{2}), (-?\d+\.\d{2}), (-?\d+\.\d{2})_?(-?\d*\.\d+), (-?\d*\.\d+), (-?\d*\.\d+), (-?\d*\.\d+)(?:_-?\d+(?:\.\d+)?)? \(\d+\)\.(png|jpg|jpeg)$`)

func ParseFilename(name string) (model.Position, error) {
	m := filenamePattern.FindStringSubmatch(filepath.Base(name))
	if m == nil {
		return model.Position{}, errors.New("filename has no EFT position metadata")
	}
	v := make([]float64, 7)
	for i := range v {
		n, err := strconv.ParseFloat(m[i+1], 64)
		if err != nil {
			return model.Position{}, err
		}
		v[i] = n
	}
	return model.Position{X: v[0], Y: v[1], Z: v[2], Rotation: QuaternionYaw(v[3], v[4], v[5], v[6]), DetectedAt: time.Now().Format(time.RFC3339Nano)}, nil
}

// QuaternionYaw mirrors EFT's coordinate convention used by tarkov.dev clients.
func QuaternionYaw(x, y, z, w float64) float64 {
	siny := 2 * (w*y + x*z)
	cosy := 1 - 2*(z*z+y*y)
	return math.Atan2(siny, cosy) * 180 / math.Pi
}
