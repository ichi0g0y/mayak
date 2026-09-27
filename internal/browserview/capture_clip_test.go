package browserview

import "testing"

func TestCaptureClipStaysWithinTheLimits(t *testing.T) {
	for _, tc := range []struct {
		name                string
		w, h, dpr           float64
		wantH, wantMaxScale float64
	}{
		{"a short page at 100%", 1280, 3000, 1, 3000, 1},
		{"a long page at 100% is cut", 1280, 40000, 1, MaxCaptureHeight, 1},
		{"a long page at 250% is scaled", 1536, 40000, 2.5, MaxCaptureHeight, 0.534},
		{"a wide page at 200%", 5000, 2000, 2, 2000, 0.82},
	} {
		w, h, scale := captureClip(tc.w, tc.h, tc.dpr)
		if h != tc.wantH || scale > tc.wantMaxScale {
			t.Fatalf("%s: clip %gx%g scale %g", tc.name, w, h, scale)
		}
		if h*tc.dpr*scale > maxCaptureDeviceHeight+1 || w*tc.dpr*scale > maxCaptureDeviceWidth+1 {
			t.Fatalf("%s: image %gx%g device px is over the limit", tc.name, w*tc.dpr*scale, h*tc.dpr*scale)
		}
	}
}
