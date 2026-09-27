package browserview

import "math"

// The largest image a whole-page capture makes, in device pixels: within
// what a snap note stores (internal/snapnote) and a canvas draws.
const (
	maxCaptureDeviceHeight = 20000
	maxCaptureDeviceWidth  = 8192
)

// captureClip is the clip of a whole-page capture for a page of cssWidth ×
// cssHeight CSS pixels on a screen of dpr device pixels per CSS pixel: its
// size in CSS pixels (the height cut at MaxCaptureHeight) and the scale that
// keeps the image within the device-pixel limits. A capture comes back in
// device pixels, so at 250% a 15000 px page would be 37500 px high.
func captureClip(cssWidth, cssHeight, dpr float64) (width, height, scale float64) {
	if dpr <= 0 || math.IsNaN(dpr) || math.IsInf(dpr, 0) {
		dpr = 1
	}
	width, height = math.Ceil(cssWidth), math.Min(math.Ceil(cssHeight), MaxCaptureHeight)
	scale = math.Min(1, math.Min(maxCaptureDeviceHeight/(height*dpr), maxCaptureDeviceWidth/(width*dpr)))
	return width, height, math.Floor(scale*1000) / 1000
}
