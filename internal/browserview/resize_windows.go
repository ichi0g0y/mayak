//go:build windows

package browserview

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

// The window is frameless, so Windows draws no sizing frame, and the page
// views are child HWNDs that take the mouse, so the Wails runtime, which
// arms resizing from the shell page, never sees the right and bottom edges
// under them. Three thin, almost transparent child windows sit over those
// edges instead: they show the size cursor and turn a press into the
// window's own resize (WM_NCLBUTTONDOWN with the edge's hit code, the
// message Wails posts for the edges it handles), so the pages can fill the
// window to its edge. They hide while the window is maximised.

const (
	edgeWidth  = 5  // CSS px along the right and bottom edges
	cornerSize = 18 // CSS px for the bottom-right square
)

type resizeEdge struct {
	hwnd   w32.HWND
	owner  w32.HWND
	hit    uintptr
	cursor w32.HCURSOR
}

// Only the UI thread touches these.
var resizeEdges = map[w32.HWND]*resizeEdge{}
var edgeClass *uint16

func edgeWndProc(hwnd w32.HWND, msg uint32, wp, lp uintptr) uintptr {
	e := resizeEdges[hwnd]
	switch msg {
	case w32.WM_SETCURSOR:
		if e != nil {
			w32.SetCursor(e.cursor)
			return 1
		}
	case w32.WM_LBUTTONDOWN:
		if e != nil {
			w32.ReleaseCapture()
			w32.PostMessage(e.owner, w32.WM_NCLBUTTONDOWN, e.hit, 0)
			return 0
		}
	case w32.WM_ERASEBKGND:
		return 1
	}
	return w32.DefWindowProc(hwnd, msg, wp, lp)
}

func ensureEdgeClass() *uint16 {
	if edgeClass != nil {
		return edgeClass
	}
	name := w32.MustStringToUTF16Ptr("MayakResizeEdge")
	class := w32.WNDCLASSEX{
		WndProc:   windows.NewCallback(edgeWndProc),
		Instance:  w32.GetModuleHandle(""),
		ClassName: name,
	}
	class.Size = uint32(unsafe.Sizeof(class))
	if w32.RegisterClassEx(&class) == 0 {
		return nil
	}
	edgeClass = name
	return name
}

// createResizeEdges makes the three edge windows for the main window.
func (m *Manager) createResizeEdges() {
	class := ensureEdgeClass()
	if class == nil || len(m.native.edges) > 0 {
		return
	}
	owner := w32.HWND(m.native.hwnd)
	for _, spec := range []struct {
		hit    uintptr
		cursor int
	}{{w32.HTRIGHT, w32.IDC_SIZEWE}, {w32.HTBOTTOM, w32.IDC_SIZENS}, {w32.HTBOTTOMRIGHT, w32.IDC_SIZENWSE}} {
		hwnd := w32.CreateWindowEx(w32.WS_EX_LAYERED, class, nil, w32.WS_CHILD|w32.WS_VISIBLE, 0, 0, 1, 1, owner, 0, w32.GetModuleHandle(""), nil)
		if hwnd == 0 {
			continue
		}
		// Alpha 1: invisible to the eye, still hit-tested.
		w32.SetLayeredWindowAttributes(hwnd, 0, 1, w32.LWA_ALPHA)
		e := &resizeEdge{hwnd: hwnd, owner: owner, hit: spec.hit, cursor: w32.LoadCursorWithResourceID(0, uint16(spec.cursor))}
		resizeEdges[hwnd] = e
		m.native.edges = append(m.native.edges, e)
	}
	m.layoutResizeEdges()
}

// layoutResizeEdges places the edge windows along the window's right and
// bottom edges, above the page views, or hides them while maximised.
func (m *Manager) layoutResizeEdges() {
	if len(m.native.edges) == 0 {
		return
	}
	owner := w32.HWND(m.native.hwnd)
	bounds := w32.GetClientRect(owner)
	if bounds == nil {
		return
	}
	if w32.IsZoomed(owner) {
		for _, e := range m.native.edges {
			w32.ShowWindow(e.hwnd, w32.SW_HIDE)
		}
		return
	}
	dpi := w32.GetDpiForWindow(owner)
	if dpi == 0 {
		dpi = 96
	}
	width, height := int(bounds.Right), int(bounds.Bottom)
	edge, corner := edgeWidth*int(dpi)/96, cornerSize*int(dpi)/96
	for _, e := range m.native.edges {
		var x, y, w, h int
		switch e.hit {
		case w32.HTRIGHT:
			x, y, w, h = width-edge, 0, edge, max(0, height-corner)
		case w32.HTBOTTOM:
			x, y, w, h = 0, height-edge, max(0, width-corner), edge
		default:
			x, y, w, h = width-corner, height-corner, corner, corner
		}
		w32.SetWindowPos(e.hwnd, w32.HWND_TOP, x, y, w, h, w32.SWP_NOACTIVATE|w32.SWP_SHOWWINDOW)
	}
}

// raiseResizeEdges keeps the edge windows above a page view that was just
// brought to the top.
func (m *Manager) raiseResizeEdges() {
	for _, e := range m.native.edges {
		w32.SetWindowPos(e.hwnd, w32.HWND_TOP, 0, 0, 0, 0, w32.SWP_NOACTIVATE|w32.SWP_NOMOVE|w32.SWP_NOSIZE)
	}
}

func (m *Manager) destroyResizeEdges() {
	for _, e := range m.native.edges {
		delete(resizeEdges, e.hwnd)
		w32.DestroyWindow(e.hwnd)
	}
	m.native.edges = nil
}
