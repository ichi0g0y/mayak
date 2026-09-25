package eftdetect

import "testing"

func TestGameLanguage(t *testing.T) {
	for input, want := range map[string]string{
		`{"Language": "en", "Other": 1}`: "en",
		`{"Language": "jp"}`:             "ja",
		`{"Language": "fr"}`:             "en",
		`{"Language": "ru"}`:             "",
		`{}`:                             "",
		`not json`:                       "",
	} {
		if got := gameLanguage([]byte(input)); got != want {
			t.Errorf("%s: got %q, want %q", input, got, want)
		}
	}
}
