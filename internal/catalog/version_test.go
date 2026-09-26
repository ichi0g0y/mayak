package catalog

import (
	"encoding/json"
	"testing"
)

func TestSnapshotVersionFollowsOnlyTheNamedResources(t *testing.T) {
	s := &Snapshot{
		Resources: map[string]json.RawMessage{"items": json.RawMessage(`{"a":1}`), "tasks": json.RawMessage(`{"t":1}`), "hideout": json.RawMessage(`{}`)},
		ETags:     map[string]string{"items": `"i1"`, "tasks": `"t1"`},
	}
	tasks, items := s.Version(TaskResources...), s.Version(ItemResources...)
	// New flea prices: the items change, the task list does not.
	s.ETags["items"] = `"i2"`
	if s.Version(TaskResources...) != tasks {
		t.Fatal("a price change changed the task list's version")
	}
	if s.Version(ItemResources...) == items {
		t.Fatal("a price change left the items' version")
	}
	// Without an ETag, the content counts.
	hideout := s.Version("hideout")
	s.Resources["hideout"] = json.RawMessage(`{"s":1}`)
	if s.Version("hideout") == hideout {
		t.Fatal("new hideout data without an ETag left its version")
	}
}
