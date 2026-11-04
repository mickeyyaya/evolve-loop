package cycleoutcome

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func missingManifestAuditRecord(t *testing.T, retro, coded bool) (root, ws string) {
	t.Helper()
	root = t.TempDir()
	ws = paths.RunWorkspace(root, 2110)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteRegistryEntry(root, "item-a", continuation.Continuation{Cycle: 2093, Branch: "cycle-2093", SnapshotSHA: "deadbeef", BaseSHA: "cafebabe"}); err != nil {
		t.Fatal(err)
	}
	gate := defectledger.New(func(string) []string { return []string{"item-a"} }, func(string, defectledger.Request) (bool, string) { return false, "stub" })
	v := gate.Reconcile(defectledger.Request{Cycle: 2110, Workspace: ws, ProjectRoot: root})
	if !v.Blocked {
		t.Fatalf("gate = %+v; want the missing manifest to block", v)
	}
	diags := append([]cyclestate.Diagnostic(nil), v.Diagnostics...)
	if !coded {
		diags[0].Code = ""
	}
	entries := []phasetiming.Entry{{Phase: "scout", Verdict: "PASS"}, {Phase: "audit", Verdict: "FAIL", Diagnostics: diags}}
	if retro {
		entries = append(entries, phasetiming.Entry{Phase: "retro", Verdict: "PASS"})
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte("Verdict: FAIL\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, ws
}

func TestFailureInputsFor_TheCodeOfAMissingManifestBlockMovesAnAuditFailToASystemLevelPhaseRefusal(t *testing.T) {
	t.Parallel()
	root, ws := missingManifestAuditRecord(t, false, true)
	_, plainWS := missingManifestAuditRecord(t, false, false)

	coded := cycleclassify.Classify(ws)
	plain := cycleclassify.Classify(plainWS)

	if coded.Class != cycleclassify.ClassPhaseRefusal || coded.Marker != string(defectledger.CodeManifestMissing) || IsTaskLevelResult(coded) {
		t.Fatalf("coded = %+v; want a phase-refusal marked %s that is not task level", coded, defectledger.CodeManifestMissing)
	}
	if plain.Class == cycleclassify.ClassPhaseRefusal || plain.Marker == string(defectledger.CodeManifestMissing) {
		t.Fatalf("plain = %+v; want the uncoded record to stay out of the refusal route", plain)
	}
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 2110, &bytes.Buffer{})
	if in.Refusal != string(defectledger.CodeManifestMissing) || !in.SystemLevel {
		t.Fatalf("inputs = refusal %q, system %v; want the refusal marker and the system level to reach the failure closeout", in.Refusal, in.SystemLevel)
	}
	if cyclestate.RefusalDisposition(in.Refusal) != (cyclestate.Disposition{}) {
		t.Errorf("disposition of %s = %+v; want none until one is added for it", in.Refusal, cyclestate.RefusalDisposition(in.Refusal))
	}
}

func TestFailureInputsFor_AMissingManifestBlockAfterTheRetroIsNotARefusal(t *testing.T) {
	t.Parallel()
	_, ws := missingManifestAuditRecord(t, true, true)

	got := cycleclassify.Classify(ws)

	if got.Class == cycleclassify.ClassPhaseRefusal {
		t.Fatalf("classification = %+v; want the retro to keep the block out of the refusal route", got)
	}
}
