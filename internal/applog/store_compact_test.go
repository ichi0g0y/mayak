package applog

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
)

// The log file keeps at most about twice the entries kept in memory.
func TestLogFileStaysBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mayak.log")
	s := &Store{limit: 10, path: path}
	for i := 0; i < 95; i++ {
		s.Add("info", "Test", "entry")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	lines := 0
	for scanner := bufio.NewScanner(file); scanner.Scan(); {
		lines++
	}
	if lines > 20 || lines < 10 {
		t.Fatalf("file has %d lines", lines)
	}
	reloaded := &Store{limit: 10, path: path}
	reloaded.load()
	if got := reloaded.Entries(); len(got) != 10 || got[9].ID != 95 {
		t.Fatalf("reloaded %d entries, last %+v", len(got), got[len(got)-1])
	}
}
