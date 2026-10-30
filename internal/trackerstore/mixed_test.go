package trackerstore

import "testing"

func TestForgetMixedProfiles(t *testing.T) {
	document := Empty()
	document.RememberProfiles([]Profile{
		{AccountID: "70", ProfileID: "pve1", Mode: "pve"},
		{AccountID: "70", ProfileID: "pve1", Mode: "seasonal"}, // mixed: forgotten
		{AccountID: "70", ProfileID: "szn1", Mode: "pvp"},      // mixed, but a key is assigned
		{AccountID: "70", ProfileID: "szn1", Mode: "seasonal"},
		{AccountID: "70", ProfileID: "old1", Mode: "pvp"}, // its logs are gone: kept
	})
	key, _ := document.AddKey("pvp", "PVP_test-token")
	if err := document.SetProfileKey("70", "szn1", "pvp", key.ID); err != nil {
		t.Fatal(err)
	}
	observed := []Profile{{AccountID: "70", ProfileID: "pve1", Mode: "pve"}, {AccountID: "70", ProfileID: "szn1", Mode: "seasonal"}}
	if !document.ForgetMixedProfiles(observed) {
		t.Fatal("nothing forgotten")
	}
	if len(document.Profiles) != 4 || document.hasProfile("70", "pve1", "seasonal") || !document.hasProfile("70", "szn1", "pvp") || !document.hasProfile("70", "old1", "pvp") {
		t.Fatalf("profiles %+v", document.Profiles)
	}
	if document.ForgetMixedProfiles(observed) {
		t.Fatal("forgot again")
	}
}
