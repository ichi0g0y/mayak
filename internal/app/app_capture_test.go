package app

import (
	"image"
	"slices"
	"testing"

	"github.com/local/mayak/internal/capture"
)

func TestCaptureKindList(t *testing.T) {
	got := captureKindList([]string{"tasks", "junk", "profile", "tasks"})
	if !slices.Equal(got, []string{"profile", "tasks"}) {
		t.Fatalf("captureKindList = %v", got)
	}
	if got := captureKindList(nil); got == nil || len(got) != 0 {
		t.Fatalf("captureKindList(nil) = %#v", got)
	}
}

func TestCaptureKindNothing(t *testing.T) {
	blank := image.NewRGBA(image.Rect(0, 0, 2560, 1440))
	if kind, crop := captureKind(blank, nil); kind != "" || crop != nil {
		t.Fatalf("a black screen = %q", kind)
	}
	// Every screen off: nothing is looked for.
	if kind, _ := captureKind(blank, captureKinds); kind != "" {
		t.Fatalf("all off = %q", kind)
	}
}

func TestCaptureTuning(t *testing.T) {
	for _, c := range []struct{ in, want [3]int }{
		{[3]int{250, 60, 30}, [3]int{250, 60, 30}},
		{[3]int{50, 0, 10}, [3]int{50, 0, 10}},
		{[3]int{2000, 3600, 100}, [3]int{2000, 3600, 100}},
		// Out of range: the default.
		{[3]int{0, -1, 0}, [3]int{250, 60, 30}},
		{[3]int{5000, 4000, 500}, [3]int{250, 60, 30}},
	} {
		s, r, p := captureTuning(c.in[0], c.in[1], c.in[2])
		if got := [3]int{s, r, p}; got != c.want {
			t.Errorf("captureTuning%v = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestCaptureTriggered(t *testing.T) {
	left, right := capture.Click{}, capture.Click{Right: true}
	shift := capture.Click{Shift: true}
	for _, c := range []struct {
		trigger string
		click   capture.Click
		want    bool
	}{
		{"right", right, true}, {"right", left, false}, {"right", shift, false},
		{"shift", shift, true}, {"shift", left, false}, {"shift", right, false},
		{"click", left, true}, {"click", right, true},
	} {
		if got := captureTriggered(c.trigger, c.click); got != c.want {
			t.Errorf("captureTriggered(%q, %+v) = %v", c.trigger, c.click, got)
		}
	}
}
