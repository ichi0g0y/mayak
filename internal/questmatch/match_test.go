package questmatch

import "testing"

func TestJapaneseAliasesKeepCanonicalIdentity(t *testing.T) {
	quests := []Quest{{ID: "ja", Name: "Delivery from the Past", Aliases: []string{"過去からの配達"}}, {ID: "other", Name: "Debut", Aliases: []string{"デビュー"}}}
	for _, text := range []string{"過去からの配達", "過 去 から の 配 達", "Ｄｅｌｉｖｅｒｙ　ｆｒｏｍ　ｔｈｅ　Ｐａｓｔ"} {
		got := Match(text, quests)
		if len(got) != 2 || got[0].Quest.ID != "ja" || got[0].Quest.Name != "Delivery from the Past" || got[0].Confidence != 1 {
			t.Fatalf("%q: %+v", text, got)
		}
	}
	if Normalize("こころ") != "こころ" {
		t.Fatal("Japanese ろ was converted to a digit")
	}
	if Similarity("配達", "過去からの配達") >= .78 {
		t.Fatal("short Japanese fragment was overconfident")
	}
}

func TestOCRConfusionMatches(t *testing.T) {
	got := Match("Sew it G00d - Part 2", []Quest{{ID: "1", Name: "Sew it Good - Part 2"}, {ID: "2", Name: "Debut"}})
	if len(got) == 0 || got[0].Quest.ID != "1" || got[0].Confidence < .95 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestNarrowUppercaseIIsTreatedLikeLowercaseL(t *testing.T) {
	got := Match("CoIIeagues", []Quest{{ID: "1", Name: "Colleagues"}, {ID: "2", Name: "Collector"}})
	if len(got) == 0 || got[0].Quest.ID != "1" || got[0].Confidence < .95 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestJapaneseThreeGlyphFromOCR(t *testing.T) {
	got := Match("Wet Job - Part ろ", []Quest{{ID: "3", Name: "Wet Job - Part 3"}, {ID: "5", Name: "Wet Job - Part 5"}})
	if len(got) == 0 || got[0].Quest.ID != "3" || got[0].Confidence != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestLeadingSeparatorReadAsI(t *testing.T) {
	got := Match("I Debtor", []Quest{{ID: "1", Name: "Debtor"}, {ID: "2", Name: "Debut"}})
	if len(got) == 0 || got[0].Quest.ID != "1" || got[0].Confidence < .99 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestRealQuestBeginningWithIStillUsesOriginalReading(t *testing.T) {
	got := Match("I Hate This Game", []Quest{{ID: "1", Name: "I Hate This Game"}, {ID: "2", Name: "Hate This Game"}})
	if len(got) == 0 || got[0].Quest.ID != "1" || got[0].Confidence != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestEmptyOCRReturnsNoCandidates(t *testing.T) {
	if got := Match("  --  ", []Quest{{ID: "1", Name: "Debut"}}); len(got) != 0 {
		t.Fatalf("unexpected candidates: %+v", got)
	}
}

func TestMatchLongSuffixDroppedByOCR(t *testing.T) {
	got := Match("ustomer", []Quest{{Name: "Big Customer"}, {Name: "Part 1"}})
	if len(got) == 0 || got[0].Quest.Name != "Big Customer" || got[0].Confidence < .82 {
		t.Fatalf("unexpected match: %+v", got)
	}
}

func TestSimilarityDoesNotBoostGenericShortFragment(t *testing.T) {
	if got := Similarity("Part 1", "Chemical - Part 1"); got >= .82 {
		t.Fatalf("short generic fragment was overconfident: %.3f", got)
	}
}
