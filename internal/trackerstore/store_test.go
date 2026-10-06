package trackerstore

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestProfileKeyBindingIsExact(t *testing.T) {
	document := Empty()
	document.RememberProfiles([]Profile{{AccountID: "account-a", ProfileID: "profile-a", Mode: "pve"}, {AccountID: "account-b", ProfileID: "profile-b", Mode: "pve"}})
	key, err := document.AddKey("pve", "PVE_test-token")
	if err != nil {
		t.Fatal(err)
	}
	if err := document.SetProfileKey("account-b", "profile-b", "pve", key.ID); err != nil {
		t.Fatal(err)
	}
	if got := document.TokenFor("account-a", "profile-a", "pve"); got != "" {
		t.Fatal("key leaked to a different EFT account")
	}
	if got := document.TokenFor("account-b", "profile-b", "pve"); got != key.Token {
		t.Fatalf("wrong token: %q", got)
	}
}

func TestKeyCannotBindAcrossModesOrProfiles(t *testing.T) {
	document := Empty()
	document.RememberProfiles([]Profile{{AccountID: "a", ProfileID: "p1", Mode: "pvp"}, {AccountID: "a", ProfileID: "p2", Mode: "pvp"}, {AccountID: "a", ProfileID: "p3", Mode: "pve"}})
	key, _ := document.AddKey("pvp", "PVP_test-token")
	if err := document.SetProfileKey("a", "p3", "pve", key.ID); err == nil {
		t.Fatal("expected mode mismatch")
	}
	if err := document.SetProfileKey("a", "p1", "pvp", key.ID); err != nil {
		t.Fatal(err)
	}
	if err := document.SetProfileKey("a", "p2", "pvp", key.ID); err == nil {
		t.Fatal("expected already-bound error")
	}
}

func TestProtectedDocumentRoundTrip(t *testing.T) {
	want := []byte(`{"version":2,"keys":[{"token":"PVP_test-token"}]}`)
	protected, err := protect(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := unprotect(protected)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("round trip mismatch: %q", got)
	}
}

func TestNamedKeysSurviveProtectedRoundTrip(t *testing.T) {
	document := Empty()
	first, err := document.AddKey("pve", "PVE_test-first", "  メイン PvE  ")
	if err != nil {
		t.Fatal(err)
	}
	second, err := document.AddKey("pve", "PVE_test-second", "メイン PvE")
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != "メイン PvE" || first.ID == second.ID {
		t.Fatal("names must be labels, not identities")
	}
	document.RememberProfiles([]Profile{{AccountID: "account", ProfileID: "profile", Mode: "pve"}})
	if err := document.SetProfileKey("account", "profile", "pve", first.ID); err != nil {
		t.Fatal(err)
	}
	plain, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	protected, err := protect(plain)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := unprotect(protected)
	if err != nil {
		t.Fatal(err)
	}
	var got Document
	if err = json.Unmarshal(restored, &got); err != nil {
		t.Fatal(err)
	}
	if got.Keys[0].Name != "メイン PvE" || got.TokenFor("account", "profile", "pve") != "PVE_test-first" || got.Keys[1].IsBound() {
		t.Fatal("name or exact key binding lost on reload")
	}
	var legacy Document
	if err = json.Unmarshal([]byte(`{"version":2,"keys":[{"id":"old","mode":"pve","token":"PVE_legacy"}]}`), &legacy); err != nil || legacy.Keys[0].Name != "" {
		t.Fatal("unnamed keys no longer load")
	}
	long, err := document.AddKey("pve", "PVE_test-long", strings.Repeat("名", 100))
	if err != nil || len([]rune(long.Name)) != 80 {
		t.Fatal("name length not bounded")
	}
}

func TestRenameBoundKeyPreservesIdentityAndPersists(t *testing.T) {
	document := Empty()
	key, _ := document.AddKey("pve", "PVE_test", "before")
	document.RememberProfiles([]Profile{{AccountID: "a", ProfileID: "p", Mode: "pve"}})
	if err := document.SetProfileKey("a", "p", "pve", key.ID); err != nil {
		t.Fatal(err)
	}
	before := document.Keys[0]
	if err := document.RenameKey(key.ID, "  編集後  "); err != nil {
		t.Fatal(err)
	}
	after := document.Keys[0]
	if after.Name != "編集後" {
		t.Fatal("name not updated")
	}
	after.Name = before.Name
	if after != before {
		t.Fatal("renaming changed identity or binding")
	}
	plain, _ := json.Marshal(document)
	encrypted, err := protect(plain)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := unprotect(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	var restored Document
	if err = json.Unmarshal(decoded, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Keys[0].Name != "編集後" || restored.TokenFor("a", "p", "pve") != key.Token {
		t.Fatal("renamed key did not survive reload")
	}
	if err := document.RenameKey("missing", "wrong"); err == nil {
		t.Fatal("missing key accepted")
	}
	if document.Keys[0].Name != "編集後" {
		t.Fatal("failed rename changed existing name")
	}
	if err := document.RenameKey(key.ID, "   "); err != nil || document.Keys[0].Name != "" {
		t.Fatal("clearing optional name failed")
	}
}

// The time a profile's past logs were synced survives the logs being read
// again (RememberProfiles only moves first and last seen).
func TestHistorySyncedAtSurvivesRememberingTheProfile(t *testing.T) {
	d := Empty()
	d.RememberProfiles([]Profile{{AccountID: "1", ProfileID: "p", Mode: "pve", FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-01-02T00:00:00Z"}})
	if !d.MarkHistorySynced("1", "p", "pve", "2026-09-27T00:00:00Z") {
		t.Fatal("profile not found")
	}
	if d.MarkHistorySynced("1", "p", "pvp", "2026-09-27T00:00:00Z") {
		t.Fatal("marked a profile of another mode")
	}
	d.RememberProfiles([]Profile{{AccountID: "1", ProfileID: "p", Mode: "pve", FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-09-26T00:00:00Z"}})
	if got := d.Profiles[0].HistorySyncedAt; got != "2026-09-27T00:00:00Z" {
		t.Fatalf("HistorySyncedAt = %q after the logs were read again", got)
	}
}

// A store keeps the file it first used: a save from work that outlived a
// test's temporary user folder must not land in the real one.
func TestStoreKeepsItsFileAfterTheFolderChanges(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	t.Setenv("APPDATA", first)
	t.Setenv("XDG_CONFIG_HOME", first)
	t.Setenv("HOME", first)
	s := &Store{}
	if _, err := s.Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", second)
	t.Setenv("XDG_CONFIG_HOME", second)
	t.Setenv("HOME", second)
	if err := s.Save(Empty()); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(second); len(entries) != 0 {
		t.Fatalf("saved into the new folder: %v", entries)
	}
}

// A Prestige is marked once: the same one read again (the live log, then
// the past logs, to the millisecond) does not make the profile wait again
// after TarkovTracker was reset; a later one does.
func TestMarkPrestige(t *testing.T) {
	d := Empty()
	d.Profiles = append(d.Profiles, Profile{AccountID: "1", ProfileID: "p", Mode: "pve"})
	if !d.MarkPrestige("1", "p", "pve", "2026-10-06T23:27:10.296Z", 353) {
		t.Fatal("first Prestige not marked")
	}
	if before, pending := d.PrestigePending("1", "p", "pve"); !pending || before != 353 {
		t.Fatalf("pending %v, %d", pending, before)
	}
	if !d.ClearPrestigePending("1", "p", "pve") || d.ClearPrestigePending("1", "p", "pve") {
		t.Fatal("the reset is recorded once")
	}
	if d.MarkPrestige("1", "p", "pve", "2026-10-06T23:27:10.296Z", 3) {
		t.Fatal("the same Prestige marked again")
	}
	if _, pending := d.PrestigePending("1", "p", "pve"); pending {
		t.Fatal("waits again for the same Prestige")
	}
	if !d.MarkPrestige("1", "p", "pve", "2026-11-01T10:00:00Z", 120) {
		t.Fatal("a later Prestige not marked")
	}
	if d.MarkPrestige("1", "other", "pve", "2026-11-02T10:00:00Z", 0) {
		t.Fatal("an unknown profile marked")
	}
}
