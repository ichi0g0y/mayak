package browserview

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Options are logical-pixel offsets inside the trusted main window.
type Options struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Left int    `json:"left"`
	Top  int    `json:"top"`
	// Right and Bottom keep strips of the window free on the right and at
	// the bottom, for the sidebars placed there.
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
	// Background ("#rrggbb") fills the view until the page paints, so a
	// switch or a slow page does not flash white.
	Background string `json:"background"`
}
type Event struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	CanBack    bool   `json:"canBack"`
	CanForward bool   `json:"canForward"`
	Popup      bool   `json:"popup"`
	Favicon    string `json:"favicon"`
	Loading    bool   `json:"loading"`
}

// Key is a key pressed in a page view with a modifier held, offered to the
// shell before the page sees it. Key is the key's name as the shell's own
// keyboard events spell it, lower-cased: a letter or digit, "tab", "pageup",
// "pagedown", "f4", "f6".
type Key struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Ctrl  bool   `json:"ctrl"`
	Shift bool   `json:"shift"`
	Alt   bool   `json:"alt"`
}
type Manager struct {
	window *application.WebviewWindow
	notify func(Event)
	native nativeManager

	scriptMu       sync.Mutex
	documentScript string
	blocker        ContentBlocker
	keys           func(Key) bool
}

// SetKeyHandler receives the modifier keys pressed in page views. A handler
// that returns true takes the key from the page; a page never sees it then.
// Only Windows forwards keys so far.
func (m *Manager) SetKeyHandler(handler func(Key) bool) {
	m.scriptMu.Lock()
	defer m.scriptMu.Unlock()
	m.keys = handler
}

func (m *Manager) keyHandler() func(Key) bool {
	m.scriptMu.Lock()
	defer m.scriptMu.Unlock()
	return m.keys
}

// ContentBlocker decides which subresources of a page are blocked, and which
// elements are hidden. kind is one of the adblock.Kind* strings.
type ContentBlocker interface {
	Block(rawURL, pageURL, kind string) bool
	CosmeticCSS(pageURL string) string
}

// SetContentBlocker applies to tabs created afterwards.
func (m *Manager) SetContentBlocker(b ContentBlocker) {
	m.scriptMu.Lock()
	defer m.scriptMu.Unlock()
	m.blocker = b
}

func (m *Manager) contentBlocker() ContentBlocker {
	m.scriptMu.Lock()
	defer m.scriptMu.Unlock()
	return m.blocker
}

// SetDocumentScript sets a script that runs before page scripts in every tab
// created afterwards. Existing tabs pick it up only when they are recreated.
func (m *Manager) SetDocumentScript(script string) {
	m.scriptMu.Lock()
	defer m.scriptMu.Unlock()
	m.documentScript = script
}

func (m *Manager) currentDocumentScript() string {
	m.scriptMu.Lock()
	defer m.scriptMu.Unlock()
	return m.documentScript
}

func New(window *application.WebviewWindow, notify func(Event)) *Manager {
	return &Manager{window: window, notify: notify}
}
func (m *Manager) Command(command string, o Options) error { return m.command(command, o) }
func (m *Manager) Close()                                  { m.close() }
