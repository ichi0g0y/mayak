package update

import "testing"

func TestParseSemver(t *testing.T) {
	cases := map[string]Semver{
		"v0.1.0":            {Minor: 1},
		"0.1.0":             {Minor: 1},
		"1.2":               {Major: 1, Minor: 2},
		"0.2.0-beta.1":      {Minor: 2, Pre: "beta.1"},
		"0.1.0-3-g1a2b3c4":  {Minor: 1, Post: 3},
		"v0.1.0-3-g1a2b3c4": {Minor: 1, Post: 3},
		"1.0.0+build.5":     {Major: 1},
	}
	for input, want := range cases {
		got, err := ParseSemver(input)
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		if got != want {
			t.Errorf("%q: got %+v, want %+v", input, got, want)
		}
	}
	for _, input := range []string{"", "dev", "v", "1", "a.b.c", "1.2.3.4", "-1.0.0"} {
		if _, err := ParseSemver(input); err == nil {
			t.Errorf("%q parsed", input)
		}
	}
}

func TestCompare(t *testing.T) {
	older := []string{"0.0.9", "0.1.0-beta.1", "0.1.0-rc.1", "0.1.0", "0.1.0-2-gabcdef0", "0.1.0-10-gabcdef0", "0.1.1", "0.2.0", "1.0.0"}
	for i := 1; i < len(older); i++ {
		a, _ := ParseSemver(older[i-1])
		b, _ := ParseSemver(older[i])
		if Compare(a, b) != -1 || Compare(b, a) != 1 || Compare(a, a) != 0 {
			t.Errorf("%s should sort before %s", older[i-1], older[i])
		}
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, candidate string
		want               bool
	}{
		{"0.1.0", "v0.1.1", true},
		{"0.1.0", "v0.1.0", false},
		{"0.1.1", "v0.1.0", false},
		{"0.1.0-3-g1a2b3c4", "v0.1.0", false},
		{"0.1.0-3-g1a2b3c4", "v0.1.1", true},
		{"0.2.0-beta.1", "v0.2.0", true},
		{"dev", "v0.1.0", true},
		{"0.1.0", "nightly", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.current, c.candidate); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v", c.current, c.candidate, got)
		}
	}
}
