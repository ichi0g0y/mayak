package position

import (
	"math"
	"testing"
)

func TestParseFilename(t *testing.T) {
	p, err := ParseFilename("2026-08-31[21-42]_123.45, -67.89, 10.00_0.00000, 0.70711, 0.00000, 0.70711 (0).png")
	if err != nil {
		t.Fatal(err)
	}
	if p.X != 123.45 || p.Y != -67.89 || p.Z != 10 {
		t.Fatalf("unexpected coordinates: %+v", p)
	}
	if math.Abs(p.Rotation-90) > .01 {
		t.Fatalf("rotation=%v", p.Rotation)
	}
}

func TestParseFilenameRejectsNormalScreenshot(t *testing.T) {
	if _, err := ParseFilename("2026-08-31[21-42] (0).png"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCurrentFilenameWithFrameTimeSuffix(t *testing.T) {
	name := "2026-09-01[16-02]_178.33, -0.36, 209.12_0.07732, 0.30418, -0.02481, 0.94915_4.28 (0).png"
	p, err := ParseFilename(name)
	if err != nil {
		t.Fatal(err)
	}
	if p.X != 178.33 || p.Y != -.36 || p.Z != 209.12 {
		t.Fatalf("unexpected: %+v", p)
	}
}
