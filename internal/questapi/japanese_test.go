package questapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/local/mayak/internal/questmatch"
	"testing"
)

type localeSource struct{ unavailable bool }

func (s localeSource) Get(_ context.Context, mode, resource string, target any) error {
	if s.unavailable {
		return errors.New("offline")
	}
	return json.Unmarshal([]byte(`{"data":{"task-name":"過去からの配達"}}`), target)
}
func TestJapaneseNamesJoinByID(t *testing.T) {
	var tasks taskEnvelope
	tasks.Data.Tasks = map[string]rawTask{"key": {ID: "stable-id", Name: "task-name"}}
	quests := []Quest{{Quest: questmatch.Quest{ID: "stable-id", Name: "Delivery from the Past"}}}
	NewWithSource(localeSource{}).addLocaleNames(context.Background(), "pve", tasks, quests)
	if len(quests[0].Aliases) != 1 || quests[0].Aliases[0] != "過去からの配達" || quests[0].Name != "Delivery from the Past" {
		t.Fatalf("%+v", quests)
	}
	NewWithSource(localeSource{unavailable: true}).addLocaleNames(context.Background(), "pve", tasks, quests)
	if len(quests[0].Aliases) != 1 {
		t.Fatal("offline lookup lost aliases")
	}
}
