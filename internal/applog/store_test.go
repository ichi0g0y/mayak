package applog

import "testing"

func TestStoreKeepsNewestEntries(t *testing.T) {
	store := &Store{limit: 2}
	store.Add("info", "test", "first")
	store.Add("warning", "test", "second")
	store.Add("error", "test", "third")

	entries := store.Entries()
	if len(entries) != 2 || entries[0].Message != "second" || entries[1].Message != "third" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	if entries[0].Level != "Warn" || entries[1].Level != "Error" {
		t.Fatalf("unexpected levels: %#v", entries)
	}
}

func TestEntriesReturnsCopy(t *testing.T) {
	store := &Store{limit: 2}
	store.Add("debug", "test", "original")
	entries := store.Entries()
	entries[0].Message = "changed"
	if store.Entries()[0].Message != "original" {
		t.Fatal("Entries exposed internal storage")
	}
}
