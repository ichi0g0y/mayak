package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLatestScanDispatchesOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "latest.png")
	if err := os.WriteFile(path, []byte("complete"), 0o600); err != nil {
		t.Fatal(err)
	}
	dispatched := make(chan string, 2)
	w, err := New(dir, func(path string) { dispatched <- path })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	w.dispatchLatestRecent(time.Minute)
	select {
	case got := <-dispatched:
		if got != path {
			t.Fatalf("got %q, want %q", got, path)
		}
	case <-time.After(time.Second):
		t.Fatal("latest screenshot was not dispatched")
	}

	w.dispatchLatestRecent(time.Minute)
	select {
	case got := <-dispatched:
		t.Fatalf("screenshot dispatched twice: %q", got)
	case <-time.After(200 * time.Millisecond):
	}

	w.mu.Lock()
	w.seen[path] = time.Now().Add(-time.Hour)
	w.mu.Unlock()
	w.dispatchLatestRecent(2 * time.Hour)
	select {
	case got := <-dispatched:
		t.Fatalf("old processed screenshot dispatched again: %q", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestCloseCancelsPendingDispatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pending.png")
	if err := os.WriteFile(path, []byte("still-writing"), 0o600); err != nil {
		t.Fatal(err)
	}
	dispatched := make(chan string, 1)
	w, err := New(dir, func(path string) { dispatched <- path })
	if err != nil {
		t.Fatal(err)
	}
	go w.waitAndDispatch(path)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-dispatched:
		t.Fatalf("dispatch occurred after close: %q", got)
	case <-time.After(250 * time.Millisecond):
	}
}
