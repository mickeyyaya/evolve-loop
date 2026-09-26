package commentaudit

import (
	"testing"
	"testing/fstest"
)

const narrated = `// Package p is tiny.
package p

// cycle-1234 showed X (F36, 2026-09-26).
// See ADR-0100 and the incident record.
// plain why: callers hold the lock.
func f() int {
	/* block
	   note */
	return 1 // trailing
}
`

func TestStats(t *testing.T) {
	got := Stats([]byte(narrated))
	want := FileStats{Code: 4, Comment: 6, Narrative: 2}
	if got != want {
		t.Fatalf("Stats = %+v, want %+v", got, want)
	}
}

func TestRank_OrdersPackagesByNarrativeThenComment(t *testing.T) {
	fsys := fstest.MapFS{
		"a/a.go":      {Data: []byte("package a\n\n// cycle-1 note\n// cycle-2 note\nfunc A() {}\n")},
		"a/a_test.go": {Data: []byte("package a\n\n// cycle-3 note\n")},
		"b/b.go":      {Data: []byte("package b\n\n// plain\n// plain\n// plain\nfunc B() {}\n")},
		"c/c.go":      {Data: []byte("package c\n\n// F12 note\nfunc C() {}\n")},
		"c/notes.txt": {Data: []byte("// cycle-9 not go\n")},
	}
	got, err := Rank(fsys)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, p := range got {
		order = append(order, p.Dir)
	}
	if want := []string{"a", "c", "b"}; !equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if got[0] != (PackageStats{Dir: "a", Files: 2, Stats: FileStats{Code: 3, Comment: 3, Narrative: 3}}) {
		t.Fatalf("package a = %+v, want both files summed", got[0])
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStats_AllowedPointersAreNotNarrative(t *testing.T) {
	src := []byte("package p\n\n// See ADR-0044.\n// See ADR-0044, ADR-0049.\n// Emits the INCIDENT envelope.\n// Widens f64 values.\n// Parses with the 2006-01-02 layout.\n// ADR-0044 came about after a long debate.\n// The incident in wave 3 showed it.\n")
	if got := Stats(src).Narrative; got != 2 {
		t.Fatalf("Narrative = %d, want 2: a See-ADR pointer, the INCIDENT envelope and f64 are not history; the retold ADR and the incident are", got)
	}
}
