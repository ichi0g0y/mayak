package iteminfo

import "testing"

func TestForFactionLeavesOutTheOtherFactionsTwin(t *testing.T) {
	info := Info{Tasks: []TaskNeed{
		{ID: "5e381b0286f77420e3417a74", Name: "Textile - Part 1", Faction: "BEAR"},
		{ID: "5e383a6386f77465910ce1f3", Name: "Textile - Part 1", Faction: "USEC"},
		{ID: "5936d90786f7742b1420ba5b", Name: "Debut"},
	}}
	usec := ForFaction(info, "USEC")
	if len(usec.Tasks) != 2 || usec.Tasks[0].Faction != "USEC" || usec.Tasks[1].Name != "Debut" {
		t.Fatalf("USEC: %+v", usec.Tasks)
	}
	if all := ForFaction(info, ""); len(all.Tasks) != 3 {
		t.Fatalf("unknown faction: %+v", all.Tasks)
	}
	if len(info.Tasks) != 3 {
		t.Fatal("the info given was changed")
	}
}
