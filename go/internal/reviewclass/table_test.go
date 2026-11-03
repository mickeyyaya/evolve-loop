package reviewclass

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var planTable = []struct {
	id    ID
	name  string
	seats []string
	cap   string
}{
	{C0, "Comment-only", nil, "$0"},
	{C1, "Docs", []string{seatDocs}, "$1"},
	{C2, "Inbox data", nil, "$0"},
	{C3, "Generated outputs", nil, "$0"},
	{C4, "Gate inputs", []string{seatCode}, "$1"},
	{C5, "Go tests only", []string{seatGoTests}, "$1"},
	{C6, "Go T2", goSeatNames, "$2"},
	{C7, "Go T1", t1SeatNames, "$4"},
	{C8, "Persona or prompt", []string{seatArchitecture, seatDocs}, "$3"},
	{C9, "Manifests and profiles", []string{seatCode, seatSecurity}, "$2"},
	{C10, "CI workflows", []string{seatCode, seatArchitecture}, "$3"},
	{C11, "T0 protected", t0SeatNames, "about $5.6 to $6.8"},
	{C12, "Stage flip", t0SeatNames, "C11"},
	{C13, "Default", goSeatNames, "$2"},
}

func TestClasses_EveryClassOfThePlanTableExists(t *testing.T) {
	t.Parallel()
	got := Classes()
	if len(got) != len(planTable) {
		t.Fatalf("Classes() has %d classes, the plan table has %d", len(got), len(planTable))
	}
	for i, want := range planTable {
		c := got[i]
		if c.ID != want.id || c.Name != want.name || c.Cap != want.cap {
			t.Errorf("class %d = %s %q cap %q, want %s %q cap %q", i, c.ID, c.Name, c.Cap, want.id, want.name, want.cap)
		}
		if seats := seatStrings(c.Seats); !slices.Equal(seats, want.seats) {
			t.Errorf("%s seats = %v, want %v", c.ID, seats, want.seats)
		}
		if c.Detect == "" || len(c.Checks) == 0 {
			t.Errorf("%s has no detection rule or no check", c.ID)
		}
		for _, check := range c.Checks {
			if check.ID == "" || check.Rule == "" {
				t.Errorf("%s has a check with no id or no rule: %+v", c.ID, check)
			}
		}
	}
}

func TestClasses_TheSecuritySeatOfT0FollowsTheGuaranteeClass(t *testing.T) {
	t.Parallel()
	want := []guards.GuaranteeClass{guards.SandboxGuarantee, guards.DispatchCredentialGuarantee, guards.PolicyGuarantee}
	for _, id := range []ID{C11, C12} {
		c, _ := Lookup(id)
		if !slices.Equal(c.SecurityWhen, want) {
			t.Errorf("%s SecurityWhen = %v, want %v", id, c.SecurityWhen, want)
		}
	}
}

func TestClasses_ACallerCannotEditTheTable(t *testing.T) {
	t.Parallel()
	got := Classes()
	if len(got) < 7 || len(got[6].Seats) == 0 {
		t.Fatalf("Classes() = %d classes; want the plan table to edit a copy of", len(got))
	}
	got[0].Name = "edited"
	got[6].Seats[0].Any[0] = "edited"
	if again := Classes(); again[0].Name == "edited" || again[6].Seats[0].Any[0] == "edited" {
		t.Error("Classes() returned the table itself; an edit of the copy changed the registry")
	}
}

func TestLookup_FindsAClassAndRefusesAnUnknownID(t *testing.T) {
	t.Parallel()
	if c, ok := Lookup(C4); !ok || c.ID != C4 {
		t.Errorf("Lookup(C4) = %v, %v", c.ID, ok)
	}
	if _, ok := Lookup("C99"); ok {
		t.Error("Lookup(C99) found a class")
	}
}

func trackedPaths(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out, err := sysexec.Output(context.Background(), sysexec.DefaultRunner, root, "git", "-c", "core.quotePath=false", "ls-files", "-z")
	if err != nil {
		t.Skipf("no git work tree: %v", err)
	}
	return strings.Split(strings.Trim(out, "\x00"), "\x00")
}

func TestKnownDefaults_EachTrackedPathHasAClassOrAListedDefault(t *testing.T) {
	t.Parallel()
	used := map[string]bool{}
	for _, p := range trackedPaths(t) {
		if ids := hitIDs(Classify(paths(p)).Classes); !slices.Equal(ids, []ID{C13}) {
			continue
		}
		pattern, ok := knownDefault(p)
		if !ok {
			t.Errorf("%s matches no class and no entry of KnownDefaults: give it a class or list it as C13 territory", p)
			continue
		}
		used[pattern] = true
	}
	for _, pattern := range KnownDefaults() {
		if !used[pattern] {
			t.Errorf("KnownDefaults entry %q matches no tracked C13 path: remove it", pattern)
		}
	}
}

func TestKnownDefault_MatchesADirectoryPrefixOrAGlob(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path string
		ok   bool
	}{
		{"knowledge-base/x/y.json", true},
		{"go/Makefile", true},
		{"go/internal/x/data.json", true},
		{"knowledge-base", false},
		{"src/x.ts", false},
	} {
		if _, ok := knownDefault(tc.path); ok != tc.ok {
			t.Errorf("knownDefault(%q) = %v, want %v", tc.path, ok, tc.ok)
		}
	}
}
