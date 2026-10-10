package profiledetect

import "testing"

func TestReadsTheLeftPanel(t *testing.T) {
	if Level(" 51\n") != 51 || Level("123") != 0 || Level("") != 0 {
		t.Fatal("level")
	}
	for _, row := range []string{"EXP+ 9 774 864", "9 774 864", "EXP 9,774,864"} {
		if n, ok := Experience(row); !ok || n != 9774864 {
			t.Fatalf("experience of %q = %d, %v", row, n, ok)
		}
	}
	// A piece with letters is not counted: the badge read as "G1" or "EXP+".
	for row, want := range map[string]int{"G1 5 200": 5200, "EXP+ 5 200": 5200, "26.3 947": 263947, "263 947": 263947, "国 263 947": 263947} {
		if n, ok := Experience(row); !ok || n != want {
			t.Fatalf("experience of %q = %d, %v, want %d", row, n, ok, want)
		}
	}
	if _, ok := Experience("ICHBOCCHI"); ok {
		t.Fatal("a name read as experience")
	}
	if _, ok := Experience("BEAR Vacation BEAR Vacation"); ok {
		t.Fatal("outfit names read as experience")
	}
	for row, want := range map[string]string{"Z7 ICHBOCCHI": "ICHBOCCHI", "ゲICHBOCCHI": "ICHBOCCHI", "Player_42": "Player_42"} {
		if got := Nickname(row); got != want {
			t.Fatalf("nickname of %q = %q, want %q", row, got, want)
		}
	}
	if Faction("BEAR") != "BEAR" || Faction("usec") != "USEC" || Faction("8EAR") != "" {
		t.Fatal("faction")
	}
}

func TestReadsTheStatsByVote(t *testing.T) {
	// How three engines read the same Japanese screenshot, line by line.
	readings := []Stats{
		ReadStats([]string{"R: 46.2", "K. 5.3.3", "L/R. 07"}, []string{"E1735: 78%", "K/D 7.51", "JT 2/7 -T 1", "2220.21h"}),
		ReadStats([]string{"462", "K: 533", "L/Rs 00/0"}, []string{"78%", "K/D. 7.51", "", "2226.21h"}),
		ReadStats([]string{"", "", "L/R の %"}, []string{"生 存 率 78 %", "K/D 7.51", "オ ン ラ イ ン", "2226.21h"}),
	}
	want := Stats{Raids: 462, Kills: 533, SurvivalRate: 78, KD: 7.51, Hours: 2226.21}
	if got := VoteStats(readings); got != want {
		t.Fatalf("voted %+v, want %+v", got, want)
	}
	// English: the time online on the third line.
	if got := ReadStats([]string{"R 462", "K 533"}, []string{"S/R 78 %", "K/D 7.51", "Online. 2226.21h"}); got != want {
		t.Fatalf("read %+v", got)
	}
	if Vote([]int{0, 51, 57, 51}) != 51 || Vote([]string{"", "ICHBOCCHI", "ICHBOCCH"}) != "ICHBOCCHI" {
		t.Fatal("vote")
	}
}

func TestLevelFromExperience(t *testing.T) {
	// tarkov.dev's first levels: each level's experience over the last.
	steps := []int{0, 1000, 3017, 4415}
	for exp, want := range map[int]int{0: 1, 999: 1, 1000: 2, 4016: 2, 4017: 3, 8432: 4, 9999999: 4} {
		if got := LevelOf(exp, steps); got != want {
			t.Fatalf("LevelOf(%d) = %d, want %d", exp, got, want)
		}
	}
}
