package scopedelta

import (
	"strings"
	"testing"
)

func TestSurfaceOf_GateImplementationsAreSignalWhateverTheirDirectory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want Surface
	}{
		{"go/internal/lane/pre_gate.go", SurfaceSignal},
		{"go/internal/lane/gatekeeper/keeper.go", SurfaceSignal},
		{"vendor/x/internal/phases/ship/stage.go", SurfaceSignal},
		{"go/internal/other/repocontract_scan.go", SurfaceSignal},
		{"go/internal/lane/lane.go", SurfaceSubject},
		{"go/internal/lane/regate.go", SurfaceSubject},
		{"docs/guide.md", SurfaceSubject},
	}
	for _, tc := range cases {
		if got := SurfaceOf(tc.path); got != tc.want {
			t.Errorf("SurfaceOf(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestAdmissible_FailsWithoutNeedsTheCommandThatWasRun(t *testing.T) {
	t.Parallel()
	keep := func(c Corroboration) Entry {
		return Entry{Path: "go/internal/lane/lane.go", Class: ClassDiscovered, Disposition: DispositionKeep, Corroboration: c}
	}

	err := Admissible(keep(Corroboration{FailsWithout: true, QueuedItemID: "item-1"}))
	if err == nil || !strings.Contains(err.Error(), "fails_without with no command") {
		t.Fatalf("err = %v", err)
	}
	if err := Admissible(keep(Corroboration{FailsWithout: true, Command: "  ", QueuedItemID: "item-1"})); err == nil {
		t.Error("a blank command is no command")
	}
	if err := Admissible(keep(Corroboration{FailsWithout: true, Command: "go test ./x", QueuedItemID: "item-1"})); err != nil {
		t.Errorf("a named command satisfies it: %v", err)
	}
	if err := Admissible(keep(Corroboration{QueuedItemID: "item-1"})); err != nil {
		t.Errorf("a queued item alone is corroboration: %v", err)
	}
}

func TestIsAtOrUnderAny_SkipsBlankRootsAndMatchesWholePathSegmentsOnly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path  string
		roots []string
		want  bool
	}{
		{"a/b.go", []string{"", "a"}, true},
		{"a/b.go", []string{"", ""}, false},
		{"a", []string{"a/"}, true},
		{"ab/c.go", []string{"a"}, false},
		{"a/b/c.go", []string{"a/b"}, true},
		{"a/b.go", nil, false},
	}
	for _, tc := range cases {
		if got := isAtOrUnderAny(tc.path, tc.roots); got != tc.want {
			t.Errorf("isAtOrUnderAny(%q,%v) = %v, want %v", tc.path, tc.roots, got, tc.want)
		}
	}
}

func TestSummarize_CountsEveryDispositionAndFlagsAMisunderstoodMajority(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		{Path: "a", Class: ClassMisunderstood, Disposition: DispositionRefuse},
		{Path: "b", Class: ClassMisunderstood, Disposition: DispositionCarve},
		{Path: "c", Class: ClassDiscovered, Disposition: DispositionKeep},
		{Path: "d", Class: ClassBoundary, Disposition: DispositionRefuse},
		{Path: "e", Class: ClassMisunderstood, Disposition: DispositionKeep},
	}

	s := Summarize(entries)

	if s.Kept != 2 || s.Carved != 1 || s.Refused != 2 {
		t.Errorf("counts kept=%d carved=%d refused=%d", s.Kept, s.Carved, s.Refused)
	}
	if s.ByClass[ClassMisunderstood] != 3 || !s.TaskStatementSuspect {
		t.Errorf("3 of 5 misunderstood is a majority: %+v", s)
	}
	half := Summarize(entries[:4])
	if half.TaskStatementSuspect {
		t.Errorf("2 of 4 is not a strict majority: %+v", half)
	}
	if empty := Summarize(nil); empty.TaskStatementSuspect || empty.Kept+empty.Carved+empty.Refused != 0 {
		t.Errorf("empty = %+v", empty)
	}
}

func TestAccount_AnInScopeClaimTheScopeDoesNotNameIsInvalid(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 9, Declared: []string{"go/internal/lane/lane.go"}}
	claim := Entry{Path: "go/internal/other/other.go", Class: ClassInScope, Disposition: DispositionKeep, Reason: "the cycle names it"}

	res := Account([]string{claim.Path}, scope, DefaultClosureRules(), []Entry{claim})

	if res.OK() || len(res.Invalid) != 1 || !strings.Contains(res.Invalid[0].Error(), "declared IN-SCOPE but the cycle's scope does not name it") {
		t.Fatalf("result = %+v", res)
	}
	honest := Entry{Path: "go/internal/lane/lane.go", Class: ClassInScope, Disposition: DispositionKeep, Reason: "named by the cycle"}
	if res := Account([]string{honest.Path}, scope, DefaultClosureRules(), []Entry{honest}); len(res.Invalid) != 0 {
		t.Errorf("a path the scope names is a legal in-scope claim: %+v", res.Invalid)
	}
}
