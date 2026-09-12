package inboxbatch

// layout_test.go — the processing/cycle-N naming is ONE belief: the writer
// (inboxmover.Claim), every reader (inboxmover.Locate, core's claim scan) and
// the parser agree by construction, not by four hand-typed literals.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProcessingCycleDir_RoundTripsWithParse(t *testing.T) {
	inbox := filepath.Join("root", ".evolve", "inbox")
	dir := ProcessingCycleDir(inbox, 1632)
	if want := filepath.Join(inbox, "processing", "cycle-1632"); dir != want {
		t.Fatalf("ProcessingCycleDir = %q, want %q", dir, want)
	}
	if filepath.Dir(dir) != ProcessingDir(inbox) {
		t.Fatalf("a cycle dir must sit directly under ProcessingDir: %q vs %q", filepath.Dir(dir), ProcessingDir(inbox))
	}
	if cycle, ok := ParseProcessingCycle(filepath.Base(dir)); !ok || cycle != 1632 {
		t.Fatalf("ParseProcessingCycle(%q) = %d,%v — must invert ProcessingCycleDir", filepath.Base(dir), cycle, ok)
	}
	for _, bad := range []string{"cycle-", "cycle-x", "processed", "cycle-12-lane"} {
		if _, ok := ParseProcessingCycle(bad); ok {
			t.Errorf("ParseProcessingCycle(%q) must reject a name that is not a plain cycle dir", bad)
		}
	}
}

// cycles start at 1 (core/alloc.go), and Location.Cycle == 0 means "pending at
// the root" — so "cycle-0" must never parse as a claim, and Claim must refuse
// to write it.
func TestParseProcessingCycle_RejectsZero(t *testing.T) {
	if _, ok := ParseProcessingCycle("cycle-0"); ok {
		t.Fatal("cycle-0 would collide with the pending sentinel; the parser must reject it")
	}
}

// ProcessingCycleDirs is the one walk every reader of the claim dirs uses
// (Locate, RecoverOrphans, dispatch state, failure counts): only parseable
// cycle dirs, ascending by cycle, never a stray file or note.
func TestProcessingCycleDirs_ListsOnlyClaimDirsInCycleOrder(t *testing.T) {
	inbox := t.TempDir()
	for _, name := range []string{"cycle-10", "cycle-9", "cycle-x", "notes"} {
		if err := os.MkdirAll(filepath.Join(ProcessingDir(inbox), name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(ProcessingDir(inbox), "cycle-11"), []byte("a file, not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ProcessingCycleDirs(inbox)
	want := []string{ProcessingCycleDir(inbox, 9), ProcessingCycleDir(inbox, 10)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProcessingCycleDirs = %v, want %v", got, want)
	}
	if got := ProcessingCycleDirs(filepath.Join(inbox, "absent")); got != nil {
		t.Fatalf("no processing dir ⇒ nil, got %v", got)
	}
}
