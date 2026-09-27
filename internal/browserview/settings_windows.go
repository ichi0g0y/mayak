//go:build windows

package browserview

// The settings the Windows views read when a tab is made (set through
// SetKeyHandler, SetContentBlocker and SetDocumentScript in view.go). Only
// Windows reads them so far.

func (m *Manager) keyHandler() func(Key) bool {
	m.scriptMu.Lock()
	defer m.scriptMu.Unlock()
	return m.keys
}

func (m *Manager) contentBlocker() ContentBlocker {
	m.scriptMu.Lock()
	defer m.scriptMu.Unlock()
	return m.blocker
}

func (m *Manager) currentDocumentScript() string {
	m.scriptMu.Lock()
	defer m.scriptMu.Unlock()
	return m.documentScript
}
