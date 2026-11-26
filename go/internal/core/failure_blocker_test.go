package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleblocker"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const (
	rawFloor2049 = buildFloorHead + "2 deterministic check failure(s) — fix these exactly before handoff:\n" +
		"  [ste-lint] the changed documents fail the STE lint: 3 finding(s) in 2 file(s) " +
		"(first: docs/explain/builds/cycle-2049-01m6cy05n0qsch7y9v4apqg24d.md:15 STE-SENTENCE); fix each finding\n" +
		"  ./acs/cycle2049 (-tags acs): unit tests FAIL\n" + bridgeNoise +
		"FAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle2049\t1.937s\n" +
		"worktree .evolve/worktrees/cycle-cd3ae73e-2049/go phase=build cycle=2049"
	rawFloor2051 = buildFloorHead + "2 deterministic check failure(s) — fix these exactly before handoff:\n" +
		"  [ste-lint] the changed documents fail the STE lint: 5 finding(s) in 1 file(s) " +
		"(first: docs/explain/builds/cycle-2051-01m6cy3x8e7tq2w9d5h4ncr0fa.md:15 STE-SENTENCE); fix each finding\n" +
		"  ./acs/cycle2051 (-tags acs): unit tests FAIL\n" + bridgeNoise +
		"FAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle2051\t2.280s\n" +
		"worktree .evolve/worktrees/cycle-0b12ef44-2051/go phase=build cycle=2051"
	dossierReason2049 = `review gate: phase "build" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff: [ste-lint] the changed documents fail`
)

func TestAttribute_Wave143Cycles2049And2051NameTheSTECheck(t *testing.T) {
	t.Parallel()
	verdict := cyclespend.Verdict{LLMMinutesToVerdict: 13.5}
	var names []string
	for _, raw := range []string{rawFloor2049, rawFloor2051} {
		got := cycleblocker.Attribute(cycleblocker.Input{CheckTags: floorCheckTags([]string{raw}), Verdict: verdict}, 50)
		if got.Blocker != "check:ste-lint" || got.Minutes != verdict.LLMMinutesToVerdict {
			t.Errorf("raw reasons: Attribute = %+v; want check:ste-lint with %v minutes", got, verdict.LLMMinutesToVerdict)
		}
		names = append(names, got.Blocker)
	}
	if names[0] != names[1] {
		t.Errorf("2049 and 2051 name %q and %q; want one shared blocker", names[0], names[1])
	}
	if tags := floorCheckTags([]string{dossierReason2049}); len(tags) != 0 {
		t.Errorf("the 200-byte dossier copy gives tags %v; only the raw reasons carry a floor line", tags)
	}
}

func TestFloorCheckTags_NonCheckBracketsAreNotTags(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"bridge token":       {"phase scout: bridge: [bridge] launch exit=81"},
		"claude-p token":     {"phase build: [claude-p] exited"},
		"defect id":          {"audit: [dd74abcdfcb2201032e8ecb889a2e5c36] defect"},
		"id on a floor line": {buildFloorHead + "1 deterministic check failure(s) — fix these exactly before handoff:\n  [dd74abcdfcb2201032e8ecb889a2e5c36] a defect"},
		"no floor head":      {"x\n  [ste-lint] not in a floor reason"},
		"empty":              nil,
	}
	for name, reasons := range cases {
		if tags := floorCheckTags(reasons); len(tags) != 0 {
			t.Errorf("%s: tags = %v; want none", name, tags)
		}
	}
}

func TestFloorCheckTags_ReturnsEachFloorTagInOrder(t *testing.T) {
	t.Parallel()
	reason := buildFloorHead + "2 deterministic check failure(s) — fix these exactly before handoff:\n  [first-check] a\n  ./acs/x: unit tests FAIL\n  [second-check] b"
	if got, want := floorCheckTags([]string{reason}), []string{"first-check", "second-check"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("floorCheckTags = %v; want %v", got, want)
	}
}

func readBlockerRecord(t *testing.T, root string, cycle int) cycleblocker.Record {
	t.Helper()
	raw, err := os.ReadFile(paths.BlockerRecordPath(filepath.Join(root, ".evolve"), cycle))
	if err != nil {
		t.Fatalf("blocker record: %v", err)
	}
	rec, err := cycleblocker.DecodeRecord(raw)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func writeLLMLedger(t *testing.T, ws string, millis int) {
	t.Helper()
	line := `{"phase":"build","cli":"claude","duration_ms":` + strconv.Itoa(millis) + `}` + "\n"
	if err := os.WriteFile(filepath.Join(ws, "llm-calls.ndjson"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAssembleFailureDigest_WritesTheBlockerRecordBesideTheDigest(t *testing.T) {
	t.Parallel()
	root, ws := layoutWorkspace(t, "2049")
	writeAuditFailReason(t, ws, "build", rawFloor2049)
	writeLLMLedger(t, ws, 810_000)

	if _, err := AssembleFailureDigest(2049, ws, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.FailureDigestRecordPath(filepath.Join(root, ".evolve"), 2049)); err != nil {
		t.Fatalf("digest record: %v", err)
	}
	want := cycleblocker.Record{Schema: cycleblocker.SchemaV1, Cycle: 2049, Blocker: "check:ste-lint", Minutes: 13.5}
	if got := readBlockerRecord(t, root, 2049); got != want {
		t.Fatalf("blocker record = %+v; want %+v", got, want)
	}
}

func TestAssembleFailureDigest_AnUntaggedFailRecordsItsClass(t *testing.T) {
	t.Parallel()
	root, ws := layoutWorkspace(t, "77")
	writeAuditFailReason(t, ws, "audit", "EGPS floor blocked ship: red_count=1 (egps-v11)")
	writeLLMLedger(t, ws, 600_000)

	if _, err := AssembleFailureDigest(77, ws, nil); err != nil {
		t.Fatal(err)
	}
	want := cycleblocker.Record{Schema: cycleblocker.SchemaV1, Cycle: 77, Blocker: "class:audit|gate-block", Minutes: 10}
	if got := readBlockerRecord(t, root, 77); got != want {
		t.Fatalf("blocker record = %+v; want %+v", got, want)
	}
}

func TestAssembleFailureDigest_AFailWithNoReasonsRecordsNone(t *testing.T) {
	t.Parallel()
	root, ws := layoutWorkspace(t, "78")

	if _, err := AssembleFailureDigest(78, ws, nil); err != nil {
		t.Fatal(err)
	}
	if got := readBlockerRecord(t, root, 78); got.Blocker != cycleblocker.None || got.Minutes != 0 {
		t.Fatalf("blocker record = %+v; want none with 0 minutes", got)
	}
}

func TestAssembleFailureDigest_AnUnwritableBlockerRecordIsAnError(t *testing.T) {
	t.Parallel()
	root, ws := layoutWorkspace(t, "5")
	if err := os.MkdirAll(paths.BlockerRecordPath(filepath.Join(root, ".evolve"), 5), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AssembleFailureDigest(5, ws, nil); !errors.Is(err, errBlockerRecord) {
		t.Fatalf("AssembleFailureDigest = %v; want the blocker record write error", err)
	}
}

func TestAssembleFailureDigest_ANonCycleWorkspaceWritesNoBlockerRecord(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeAuditFailReason(t, ws, "build", rawFloor2049)
	if _, err := AssembleFailureDigest(1, ws, nil); err != nil {
		t.Fatal(err)
	}
	if matches, _ := filepath.Glob(filepath.Join(ws, "..", "*", "*.blocker.json")); len(matches) != 0 {
		t.Fatalf("blocker records written outside a harness layout: %v", matches)
	}
}

func TestAssembleFailureDigest_AnUnreadableLedgerIsABlockerRecordError(t *testing.T) {
	t.Parallel()
	_, ws := layoutWorkspace(t, "6")
	if err := os.Mkdir(filepath.Join(ws, "llm-calls.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AssembleFailureDigest(6, ws, nil); !errors.Is(err, errBlockerRecord) {
		t.Fatalf("AssembleFailureDigest = %v; want the blocker record error for a ledger that cannot be read", err)
	}
}

func TestAssembleFailureDigest_ABlockerRecordThatCannotBeEncodedIsAnError(t *testing.T) {
	orig := blockerMarshal
	blockerMarshal = func(any) ([]byte, error) { return nil, errors.New("encoder refused") }
	t.Cleanup(func() { blockerMarshal = orig })
	_, ws := layoutWorkspace(t, "7")
	if _, err := AssembleFailureDigest(7, ws, nil); !errors.Is(err, errBlockerRecord) {
		t.Fatalf("AssembleFailureDigest = %v; want the blocker record error", err)
	}
}
