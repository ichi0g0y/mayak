//go:build windows || darwin

package browserview

import (
	_ "embed"
	"encoding/json"
)

// collapseScript hides blocked elements and the empty slots they leave.
//
//go:embed collapse.js
var collapseScript string

// cosmeticScript adds the element hiding stylesheet once per document.
const cosmeticScript = `(css=>{const id="mayak-cosmetic";if(document.getElementById(id))return;
const style=document.createElement("style");style.id=id;style.textContent=css;
(document.head||document.documentElement).appendChild(style);
if(window.__mayakTidy)window.__mayakTidy();})`

// cosmeticCall is the script that adds css (from ContentBlocker.CosmeticCSS)
// to a page.
func cosmeticCall(css string) string {
	data, _ := json.Marshal(css)
	return cosmeticScript + "(" + string(data) + ")"
}
