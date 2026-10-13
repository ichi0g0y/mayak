package app

import "testing"

func TestGameInputGoesToClientsOnlyWhenSetAndOneIsConnected(t *testing.T) {
	a := &App{}
	if a.gameAway() {
		t.Fatal("off and alone")
	}
	a.BrowserLinkClients(2)
	if a.gameAway() {
		t.Fatal("GameRemote is off")
	}
	a.wheel.remote.Store(true)
	if !a.gameAway() {
		t.Fatal("on with Clients connected")
	}
	a.BrowserLinkClients(0)
	if a.gameAway() {
		t.Fatal("no Client connected")
	}
	a.BrowserLinkClients(-3)
	if a.wheel.clients.Load() != 0 {
		t.Fatalf("clients = %d", a.wheel.clients.Load())
	}
}

func TestBrowserGameScrollWithoutViews(t *testing.T) {
	if (&App{}).BrowserGameScroll(120) {
		t.Fatal("no page views to scroll")
	}
}
