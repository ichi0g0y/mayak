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
