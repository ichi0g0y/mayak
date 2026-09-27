package locale

import "testing"

func TestResource(t *testing.T) {
	if got := Resource("items", "ja"); got != "items_ja" {
		t.Errorf("Resource = %q, want items_ja", got)
	}
}

func TestLanguages(t *testing.T) {
	seen := map[string]bool{}
	for _, lang := range Languages {
		// English is the catalog's own language, never an extra one.
		if lang == "" || lang == "en" || seen[lang] {
			t.Errorf("language %q is empty, English or listed twice", lang)
		}
		seen[lang] = true
	}
}
