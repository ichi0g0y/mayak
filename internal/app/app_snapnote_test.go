package app

import "testing"

func TestExportName(t *testing.T) {
	for title, want := range map[string]string{
		"Customs - Extracts":    "Customs - Extracts",
		` a/b\c:d*e?"f"<g>|h. `: "a_b_c_d_e__f__g__h",
		"":                      "snapnote",
		"...":                   "snapnote",
		"税関\tメモ":                "税関_メモ",
	} {
		if got := exportName(title); got != want {
			t.Errorf("exportName(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestValidMapName(t *testing.T) {
	for name, want := range map[string]bool{"customs": true, "streets-of-tarkov": true, "ground-zero-21": true, "": false, "Customs": false, "../x": false} {
		if got := validMapName(name); got != want {
			t.Errorf("validMapName(%q) = %v", name, got)
		}
	}
}
