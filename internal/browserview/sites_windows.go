//go:build windows

package browserview

import _ "embed"

// siteScript tidies some sites' pages in every tab, the ad blocker on or
// off (sites.js).
//
//go:embed sites.js
var siteScript string
