// Package locale lists the game languages MAYAK reads besides English.
//
// Their names come from tarkov.dev's per-language resources (items_ja,
// tasks_ja, ...), which hold the game's own text. They are fetched with the
// catalog, matched against titles read from a game in that language, and
// shown when the shell's language is that one. Adding a language here adds
// all of that; the shell's words and an OCR model are separate steps (see
// docs/languages.md).
package locale

// Languages are tarkov.dev's language codes, in order of preference.
var Languages = []string{"ja"}

// Resource is a catalog resource's name in lang, e.g. items_ja.
func Resource(base, lang string) string { return base + "_" + lang }
