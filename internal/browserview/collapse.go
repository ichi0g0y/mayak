//go:build windows || darwin

package browserview

import (
	_ "embed"
	"encoding/json"
)

// collapseScript hides blocked elements and the empty slots they leave,
// starting from what MAYAK hid (looked up directly, not every element).
//
//go:embed collapse.js
var collapseScript string

// cosmeticScript adds the element hiding stylesheet once per document.
//
//go:embed cosmetic.js
var cosmeticScript string

// cosmeticCall is the script that adds css and generic (from
// ContentBlocker.CosmeticCSS) to a page.
func cosmeticCall(css, generic string) string {
	cssData, _ := json.Marshal(css)
	genericData, _ := json.Marshal(generic)
	return "(" + cosmeticScript + ")(" + string(cssData) + "," + string(genericData) + ")"
}
