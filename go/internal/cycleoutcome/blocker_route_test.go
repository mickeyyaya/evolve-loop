package cycleoutcome

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func TestApplyFailure_AProvenBuildBlockerRoutesTheItemConsoleWithTheDiagnosis(t *testing.T) {
	root, inbox, ws := seedProject(t, "seam", []string{"seam"})
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "seam", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	entries := []phasetiming.Entry{
		{Phase: "tdd", Verdict: "PASS"},
		{Phase: "build", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{
			Severity: cyclestate.SeverityError, Code: cyclestate.DiagCodeBlockerRoute, Subject: "seam",
			Message: "blocked: needs console — phase build stopped on a proven blocker the lane cannot remove: protected-path: the fix needs go/internal/core/phase_bindings_selfcheck.go",
		}}},
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(phasetiming.Path(ws), data, 0o644); err != nil {
		t.Fatal(err)
	}
	laneFail := "<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\",\"schema_version\":2,\"failure\":{\"class\":\"code-build-fail\",\"defects\":[\"BLOCKER protected-path go/internal/core/phase_bindings_selfcheck.go why\"]}} -->\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(laneFail), 0o644); err != nil {
		t.Fatal(err)
	}

	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard)
	if in.SystemLevel || in.Refusal != cyclestate.DiagCodeBlockerRoute || in.RefusalSubject != "seam" {
		t.Fatalf("inputs = %+v; the proven blocker, not the lane's FAIL sentinel, classifies the cycle", in)
	}
	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}

	rec := itemRecord(t, inbox, "seam")
	if rec["route"] != "console-manual" || !strings.Contains(rec["routed_reason"].(string), "phase_bindings_selfcheck.go") {
		t.Errorf("item = %v; the console gets the item with the diagnosis", rec)
	}
	if _, bumped := rec["failure_count"]; bumped {
		t.Errorf("a blocker is not a code FAIL, so no failure_count: %v", rec)
	}
}
