package cycleoutcome

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func failBuildBeforeTheRetro(t *testing.T, code, message string) string {
	t.Helper()
	root, inbox, ws := seedProject(t, "seam", []string{"seam"})
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "seam", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	entries := []phasetiming.Entry{
		{Phase: "scout", Verdict: "PASS"},
		{Phase: "tdd", Verdict: "PASS"},
		{Phase: "build", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Code: code, Subject: "seam", Message: message}}},
		{Phase: string(cyclestate.PhaseRetro), Verdict: "PASS"},
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(phasetiming.Path(ws), data, 0o644); err != nil {
		t.Fatal(err)
	}
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard)
	if in.SystemLevel || in.Refusal != code {
		t.Fatalf("inputs = %+v; the build's coded entry, not the retro's PASS, classifies the cycle", in)
	}
	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	return inbox
}

func TestApplyFailure_ABuildRejectedAfterItsCorrectionsChargesTheItem(t *testing.T) {
	inbox := failBuildBeforeTheRetro(t, cyclestate.DiagCodeBuildRejected, `review gate: phase "build" deliverable rejected after 2 correction(s): build handoff floor`)

	rec := itemRecord(t, inbox, "seam")
	if rec["failure_count"] != float64(1) || rec["route"] != nil {
		t.Fatalf("item = %v; a build that could not meet a satisfiable contract charges the item and keeps it on the lanes", rec)
	}
}

func TestApplyFailure_ADefectivePredicateBeforeTheRetroRoutesTheItemAndDoesNotChargeIt(t *testing.T) {
	inbox := failBuildBeforeTheRetro(t, cyclestate.DiagCodeBlockerRoute, "blocked: needs console — predicate-conflict: TestX fails at go/x.go and TestGuard forbids the edit")

	rec := itemRecord(t, inbox, "seam")
	if _, bumped := rec["failure_count"]; bumped || rec["route"] != "console-manual" {
		t.Fatalf("item = %v; a proven predicate defect is the pipeline's, so the item goes to the console uncharged", rec)
	}
}
