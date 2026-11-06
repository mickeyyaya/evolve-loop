package core

import (
	"maps"
	"os"
	"path/filepath"
	"testing"
)

func TestPredicateSnapshot_RoundTripsAndReturnsItsFaultApartFromTheMap(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	path := filepath.Join(ws, predicateSnapshotFile)
	got, fault := readPredicateSnapshot(ws)
	if len(got) != 0 || fault != "no predicate base at "+path+" for this Build entry" {
		t.Errorf("a missing snapshot reads as %v, %q", got, fault)
	}
	want := map[string]string{"a_test.go": "package a\n"}
	if err := writePredicateSnapshot(ws, want); err != nil {
		t.Fatal(err)
	}
	if got, fault := readPredicateSnapshot(ws); !maps.Equal(got, want) || fault != "" {
		t.Errorf("round trip = %v, %q; want %v", got, fault, want)
	}
	if err := writePredicateSnapshot(ws, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if got, fault := readPredicateSnapshot(ws); len(got) != 0 || fault != "" {
		t.Errorf("a sealed empty base reads as %v, %q; no predicate file is a base, not a fault", got, fault)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, fault = readPredicateSnapshot(ws)
	wantFault := "the predicate base at " + path + " does not parse: unexpected end of JSON input"
	if len(got) != 0 || fault != wantFault {
		t.Errorf("a corrupt snapshot reads as %v, %q; want an empty map and %q", got, fault, wantFault)
	}
	if err := writePredicateSnapshot(filepath.Join(ws, "missing"), want); err == nil {
		t.Error("writing into a missing workspace must fail loudly")
	}
}

func TestResumedPredicateBase_KeepsTheLastSealedBaseOrSealsTheEntryTree(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	wt := t.TempDir()
	dir := filepath.Join(wt, "go", "acs", "cycle4")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p_test.go"), []byte("now"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePredicateSnapshot(ws, map[string]string{"p_test.go": "kept"}); err != nil {
		t.Fatal(err)
	}
	cs := CycleState{WorkspacePath: ws, ActiveWorktree: wt}

	kept, err := keptPredicateBase(cs, 4)
	if err != nil || kept.sources["p_test.go"] != "kept" || kept.fault != "" {
		t.Fatalf("keep = %+v, %v; the first entry of a resume reads the base that was sealed", kept, err)
	}
	sealed, err := sealedPredicateBase(cs, 4)
	if err != nil || sealed.sources["p_test.go"] != "now" {
		t.Fatalf("seal = %+v, %v; a later entry takes the tree of that entry as the base", sealed, err)
	}
	if again, fault := readPredicateSnapshot(ws); again["p_test.go"] != "now" || fault != "" {
		t.Errorf("the seal is stored: %v, %q", again, fault)
	}

	empty := CycleState{WorkspacePath: t.TempDir()}
	missing, err := keptPredicateBase(empty, 4)
	if err != nil || missing.fault == "" {
		t.Errorf("keep with no stored base = %+v, %v; the fault travels with the base", missing, err)
	}
	if _, err := sealedPredicateBase(CycleState{WorkspacePath: filepath.Join(ws, "missing")}, 4); err == nil {
		t.Error("a seal that cannot be stored must fail loudly")
	}
}

func TestSealPredicateBase_NeedsNoWorkspaceAndFailsLoudlyOnAWriteError(t *testing.T) {
	t.Parallel()
	base, err := sealPredicateBase("", "", 4)
	if err != nil || base == nil {
		t.Fatalf("err=%v base=%v; an empty workspace still takes the in-memory base", err, base)
	}
	if _, err := sealPredicateBase(filepath.Join(t.TempDir(), "missing"), "", 4); err == nil {
		t.Error("a base that cannot be stored must stop the cycle, not pass silently")
	}
}

func TestWithBlockerRefusal_AppendsTheRefusalOnlyWhenThereIsOne(t *testing.T) {
	t.Parallel()
	if got := withBlockerRefusal("reason", ""); got != "reason" {
		t.Errorf("got %q", got)
	}
	if got := withBlockerRefusal("reason", "why"); got != "reason\n\nThe host refused your BLOCKER claim: why" {
		t.Errorf("got %q", got)
	}
}
