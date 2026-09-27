package sysfonts

import (
	"reflect"
	"runtime"
	"testing"
)

func TestClean(t *testing.T) {
	got := clean([]string{"Meiryo", "@MS Gothic", "arial", "Meiryo", " ", "MS Gothic", "Arial Black"})
	want := []string{"arial", "Arial Black", "Meiryo", "MS Gothic"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clean = %q, want %q", got, want)
	}
}

func TestList(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}
	fonts := List()
	has := map[string]bool{}
	for _, f := range fonts {
		has[f] = true
	}
	// Every Windows has these.
	if !has["Arial"] || !has["Segoe UI"] {
		t.Fatalf("%d fonts, Arial %v, Segoe UI %v", len(fonts), has["Arial"], has["Segoe UI"])
	}
}
