// Types the shell's JavaScript is checked with (tsconfig.shell.json).
//
// window.mayak is the shell's API (api.js); window.mayakDesktop the bridge
// the settings page and the views use. The shell reads event targets and
// the elements it queries as the HTML elements they are (inputs, forms,
// canvases); rather than a cast at each of them, their members are loose
// here. Names, imports and calls are still checked.
interface Window {
  mayak: any
  mayakDesktop: any
}
interface EventTarget {
  [member: string]: any
}
interface Element {
  [member: string]: any
}
