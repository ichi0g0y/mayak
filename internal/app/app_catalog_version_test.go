package app

import "testing"

func TestCatalogChangedOnlyOnANewVersion(t *testing.T) {
	a := &App{}
	if !a.catalogChanged("pve", "tasks", "v1") {
		t.Fatal("the first version is a change")
	}
	if a.catalogChanged("pve", "tasks", "v1") {
		t.Fatal("the same version again is no change")
	}
	if !a.catalogChanged("regular", "tasks", "v1") {
		t.Fatal("another mode keeps its own version")
	}
	if !a.catalogChanged("pve", "tasks", "v2") {
		t.Fatal("a new version is a change")
	}
}

// A version is only seen once it is remembered: the hideout remembers its
// version after its stations were read, so a failed read is tried again.
func TestCatalogSeenOnlyOnceRemembered(t *testing.T) {
	a := &App{}
	if a.catalogSeen("pve", "hideout", "v1") {
		t.Fatal("seen before anything was remembered")
	}
	// A failed read remembers nothing: the next refresh reads again.
	if a.catalogSeen("pve", "hideout", "v1") {
		t.Fatal("seen after a failed read")
	}
	a.catalogRemember("pve", "hideout", "v1")
	if !a.catalogSeen("pve", "hideout", "v1") || a.catalogSeen("pve", "hideout", "v2") {
		t.Fatal("remembered version not seen, or another one seen")
	}
}
