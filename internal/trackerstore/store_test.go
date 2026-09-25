package trackerstore

import (
	"encoding/json"
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
