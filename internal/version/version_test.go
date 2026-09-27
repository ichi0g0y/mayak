package version

import "testing"

func TestCurrent(t *testing.T) {
	defer func(saved string) { Version = saved }(Version)
	for _, test := range []struct {
		version, current string
		development      bool
	}{
		{"v0.1.17", "0.1.17", false},
		{"0.1.17-48-g55c7de5", "0.1.17-48-g55c7de5", false},
		{" v0.2.0\n", "0.2.0", false},
		{Development, Development, true},
		{"", "", true},
	} {
		Version = test.version
		if got := Current(); got != test.current {
			t.Errorf("Current() with %q = %q, want %q", test.version, got, test.current)
		}
		if got := IsDevelopment(); got != test.development {
			t.Errorf("IsDevelopment() with %q = %v, want %v", test.version, got, test.development)
		}
	}
	Version = "v1.2.3"
	if got := UserAgent(); got != "MAYAK/1.2.3" {
		t.Errorf("UserAgent() = %q", got)
	}
}
