package carryover

// rules_test.go — unit 03 (ADR-0103, design decomposition/03-carryover-lifecycle.md):
// the pure rules — identity, unions, retirement, expiry, caps, the summary and
// the priority vocabulary. RED first; every rule is byte-for-byte the
// pre-extraction behaviour.

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestFingerprint_NormalizesCycleTokens(t *testing.T) {
	a := fingerprint("cycle 1421 failed during audit: predicate tree includes undeclared inputs")
	b := fingerprint("cycle 1428 failed during audit: predicate tree includes undeclared inputs")
	if a != b || a == "" {
		t.Fatalf("both mint spellings collapse across cycles: %q vs %q", a, b)
	}
	if fingerprint("Fix defect from cycle 1424: x") != fingerprint("Fix defect from cycle 1427:  x") {
		t.Fatal("the defect spelling collapses across cycles and double spaces fold")
	}
	if fingerprint("CYCLE-7 failed") != fingerprint("cycle 7 failed") {
		t.Fatal("case-insensitive, both separators")
	}
	if fingerprint("Cycle 1 Failed During Build") != fingerprint("cycle 1 failed during build") {
		t.Fatal("the whole action is lower-cased, not only the cycle token")
	}
	if fingerprint("cycle 1 failed during audit: a") == fingerprint("cycle 1 failed during audit: b") {
		t.Fatal("distinct classes stay distinct")
	}
	if fingerprint("") != "" {
		t.Fatal("empty stays empty")
	}
}

func TestFingerprintIndex_ReturnsTheFirstTwinOrMinusOne(t *testing.T) {
	todos := []cyclestate.CarryoverTodo{{ID: "a", Action: "cycle 1 failed during build: x"}, {ID: "b", Action: "cycle 2 failed during build: x"}}
	if got := fingerprintIndex(todos, "cycle 9 failed during build: x"); got != 0 {
		t.Fatalf("the FIRST twin wins: %d", got)
	}
	if got := fingerprintIndex(todos, "cycle 9 failed during audit: y"); got != -1 {
		t.Fatalf("no twin: %d", got)
	}
	if got := fingerprintIndex(nil, "anything"); got != -1 {
		t.Fatalf("nil: %d", got)
	}
}

func TestHasID_ScansByID(t *testing.T) {
	todos := []cyclestate.CarryoverTodo{{ID: "a"}, {ID: "b"}}
	if !HasID(todos, "b") || HasID(todos, "c") || HasID(nil, "a") {
		t.Fatal("present / absent / nil")
	}
}

func TestMergeTodos_UnionsByIDDiskFirstAndKeepsTheDiskCopy(t *testing.T) {
	disk := []cyclestate.CarryoverTodo{{ID: "shared", Action: "disk", ExpiresAt: "2026-01-01T00:00:00Z"}, {ID: "d1"}}
	incoming := []cyclestate.CarryoverTodo{{ID: "shared", Action: "incoming", ExpiresAt: "2026-02-01T00:00:00Z"}, {ID: "i1"}, {ID: "i2"}}
	got := MergeTodos(disk, incoming)
	if len(got) != 4 || got[0].Action != "disk" || got[0].ExpiresAt != "2026-01-01T00:00:00Z" || got[1].ID != "d1" || got[2].ID != "i1" || got[3].ID != "i2" {
		t.Fatalf("ID-only, disk-first, the DISK copy kept byte-for-byte, incoming appended in order: %+v", got)
	}
	same := MergeTodos(disk, nil)
	same[0].Action = "mutated"
	if disk[0].Action != "disk" {
		t.Fatal("the result never aliases the disk slice (the nil-append copy), even when nothing is appended")
	}
}

func TestMergeFailedRecords_UnionsDiskAndIncoming(t *testing.T) {
	disk := []cyclestate.FailedRecord{{Cycle: 1, TS: "t1", Verdict: "FAIL", RecordedAt: "r1", Summary: "disk"}}
	incoming := []cyclestate.FailedRecord{{Cycle: 1, TS: "t1", Verdict: "FAIL", RecordedAt: "r1", Summary: "incoming"}, {Cycle: 2, TS: "t2", Verdict: "FAIL", RecordedAt: "r2"}}
	got := MergeFailedRecords(disk, incoming)
	if len(got) != 2 || got[0].Summary != "incoming" || got[1].Cycle != 2 {
		t.Fatalf("incoming wins for a shared key, disk order kept: %+v", got)
	}
}

func TestMergeFailedRecords_KeyIsCycleTSVerdictRecordedAt(t *testing.T) {
	base := cyclestate.FailedRecord{Cycle: 1, TS: "t", Verdict: "FAIL", RecordedAt: "r"}
	ts, verdict, recorded, same := base, base, base, base
	ts.TS, verdict.Verdict, recorded.RecordedAt = "t2", "WARN", "r2"
	got := MergeFailedRecords([]cyclestate.FailedRecord{base}, []cyclestate.FailedRecord{ts, verdict, recorded, same})
	if len(got) != 4 {
		t.Fatalf("records differing only in TS / Verdict / RecordedAt are three keys; identical tuples collapse: %d", len(got))
	}
}

func TestRetire_CommittedIDRetires(t *testing.T) {
	todos := []cyclestate.CarryoverTodo{{ID: "a", Action: "x"}, {ID: "b", Action: "y"}}
	got := Retire(todos, []string{" a "})
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("a committed id (trimmed) retires: %+v", got)
	}
}

func TestRetire_FingerprintVariantRetires(t *testing.T) {
	todos := []cyclestate.CarryoverTodo{{ID: "cycle-1-failed-build", Action: "cycle 1 failed during build: x"}, {ID: "cycle-2-failed-build", Action: "cycle 2 failed during build: x"}, {ID: "keep", Action: "other"}}
	got := Retire(todos, []string{"cycle-1-failed-build"})
	if len(got) != 1 || got[0].ID != "keep" {
		t.Fatalf("a committed todo retires its cross-cycle fingerprint twins too: %+v", got)
	}
}

func TestRetire_UnmatchedSurvivesInOrder(t *testing.T) {
	todos := []cyclestate.CarryoverTodo{{ID: "a", Action: "1"}, {ID: "b", Action: "2"}, {ID: "c", Action: "3"}}
	got := Retire(todos, []string{"b"})
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Fatalf("survivors keep their order: %+v", got)
	}
}

func TestRetire_EdgeInputs(t *testing.T) {
	if got := Retire(nil, []string{"a"}); len(got) != 0 {
		t.Fatalf("nil todos: %+v", got)
	}
	todos := []cyclestate.CarryoverTodo{{ID: "a", Action: ""}, {ID: "b", Action: ""}}
	if got := Retire(todos, []string{"a"}); len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("a committed EMPTY-Action entry must not retire an uncommitted empty-Action entry: %+v", got)
	}
	if got := Retire(todos, []string{"", "  "}); len(got) != 2 {
		t.Fatalf("blank ids retire nothing: %+v", got)
	}
	blankID := []cyclestate.CarryoverTodo{{ID: "", Action: "unnamed"}, {ID: "b", Action: "named"}}
	if got := Retire(blankID, []string{"  "}); len(got) != 2 {
		t.Fatalf("a blank committed id never matches a todo with an empty id: %+v", got)
	}
}

func TestRetire_DoesNotMutateInput(t *testing.T) {
	todos := []cyclestate.CarryoverTodo{{ID: "a", Action: "x"}}
	early := Retire(todos, nil)
	early[0].Action = "mutated"
	if todos[0].Action != "x" {
		t.Fatal("the early return is a copy, never the caller's slice")
	}
	full := Retire(todos, []string{"nomatch"})
	full[0].ID = "mutated"
	if todos[0].ID != "a" {
		t.Fatal("the full path returns a new slice")
	}
}

func TestRefreshExpiry_LaterStampWinsEqualAndEmptyDoNot(t *testing.T) {
	td := cyclestate.CarryoverTodo{ExpiresAt: "2026-01-02T00:00:00Z"}
	if !refreshExpiry(&td, "2026-01-03T00:00:00Z") || td.ExpiresAt != "2026-01-03T00:00:00Z" {
		t.Fatal("a later Z stamp overwrites and reports the write")
	}
	if refreshExpiry(&td, "2026-01-03T00:00:00Z") || refreshExpiry(&td, "2026-01-01T00:00:00Z") || refreshExpiry(&td, "") || td.ExpiresAt != "2026-01-03T00:00:00Z" {
		t.Fatal("equal, earlier and empty stamps never win (the UTC second-precision string compare)")
	}
}

func TestCapRunes_TruncatesWithEllipsisAtTheBoundary(t *testing.T) {
	if MaxActionRunes != 500 {
		t.Fatalf("MaxActionRunes = %d", MaxActionRunes)
	}
	s499, s500, s501 := strings.Repeat("a", 499), strings.Repeat("b", 500), strings.Repeat("c", 501)
	if CapRunes(s499, 500) != s499 || CapRunes(s500, 500) != s500 || CapRunes(s501, 500) != s500[:0]+strings.Repeat("c", 500)+"…" {
		t.Fatal("499/500 untouched; 501 → 500 + the ellipsis rune")
	}
}

func TestSummary_CapsTheMessageWithTheTruncationMarker(t *testing.T) {
	if MaxSummaryRunes != 500 {
		t.Fatalf("MaxSummaryRunes = %d", MaxSummaryRunes)
	}
	if got := Summary(281, "build", errors.New("boom")); got != "cycle 281 failed during build: boom" {
		t.Fatalf("the failure shape: %q", got)
	}
	long := strings.Repeat("x", 600)
	got := Summary(1, "audit", errors.New(long))
	if !strings.HasSuffix(got, strings.Repeat("x", 500)+" ...[truncated]") || strings.Contains(got, "…") {
		t.Fatalf("Summary's marker is \" ...[truncated]\", not CapRunes' ellipsis — two rules, both preserved: %q", got[len(got)-30:])
	}
}

func TestPriorities_AreTheProducersSpellings(t *testing.T) {
	if PriorityBlocking != "P0" || PriorityLesson != "P1" || PriorityPrescription != "high" || PriorityMemoDefault != "medium" || PrescriptionPrefix != "PRESCRIPTION: " {
		t.Fatalf("the vocabulary the producers spell: %q %q %q %q %q", PriorityBlocking, PriorityLesson, PriorityPrescription, PriorityMemoDefault, PrescriptionPrefix)
	}
}

// ADR-0103 unit 03b: the THIRD rune cap — the advisor prompt's and the
// remediation title's — moved beside CapRunes and Summary so the three rules
// are enumerable in one file. Distinct marker from both (the F5 twin).
func TestTruncateRunes_TrimsCapsAndMarks(t *testing.T) {
	long := strings.Repeat("é", 501)
	if got := TruncateRunes("  "+strings.Repeat("é", 499)+"  ", 500); got != strings.Repeat("é", 499) {
		t.Fatalf("under the cap: trimmed, untouched: %q", got)
	}
	if got := TruncateRunes(strings.Repeat("é", 500), 500); got != strings.Repeat("é", 500) {
		t.Fatal("at the cap: untouched")
	}
	got := TruncateRunes(long, 500)
	if got != strings.Repeat("é", 500)+" …[truncated]" {
		t.Fatalf("over the cap: 500 runes + the exact marker, got %d runes ending %q", len([]rune(got)), got[len(got)-14:])
	}
	if got == CapRunes(long, 500) || got == Summary(1, "x", errors.New(long)) {
		t.Fatal("the three caps are three rules: TruncateRunes' marker differs from CapRunes' and Summary's")
	}
}
