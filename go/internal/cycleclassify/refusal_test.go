package cycleclassify

// refusal_test.go — the C1-record pass: a cycle whose LAST recorded phase
// outcome is a FAIL carrying a diagnostic Code is a phase-refusal — the phase's
// own deterministic gate stopped the cycle (triage refusing a protected-surface
// card), which is task-attributable and re-occurs on every retry. Read from
// phase-timing.json, never from prose; ranked after the agent's sentinel class
// (pass 0) and before the regex passes, because a coded refusal IS the reason
// the cycle stopped (docs/incidents/2026-09-14-triage-refusal-poison-loop.md).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func writePhaseTiming(t *testing.T, ws string, entries []phasetiming.Entry) {
	t.Helper()
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(phasetiming.Path(ws), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func refusalEntries(code string) []phasetiming.Entry {
	return []phasetiming.Entry{
		{Phase: "scout", Verdict: "PASS"},
		{Phase: "triage", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Message: "top_n card names protected surface", Code: code}}},
	}
}

func TestClassify_CodedRefusalOnTheLastOutcome_IsPhaseRefusal(t *testing.T) {
	ws := t.TempDir() // no orchestrator-report.md: the abnormal path writes none
	writePhaseTiming(t, ws, refusalEntries(cyclestate.DiagCodeTriageProtectedSurface))
	r := Classify(ws)
	if r.Class != ClassPhaseRefusal || r.Marker != cyclestate.DiagCodeTriageProtectedSurface || r.Source != phasetiming.FileName {
		t.Errorf("Classify = %+v", r)
	}
}

func TestClassify_UncodedFailOnTheRecord_IsNotARefusal(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, refusalEntries(""))
	if r := Classify(ws); r.Class == ClassPhaseRefusal {
		t.Errorf("an uncoded FAIL is prose, not a refusal: %+v", r)
	}
}

func TestClassify_RefusalMustBeTheLastOutcome(t *testing.T) {
	ws := t.TempDir()
	entries := append(refusalEntries(cyclestate.DiagCodeTriageTopNEmpty), phasetiming.Entry{Phase: "build", Verdict: "PASS"})
	writePhaseTiming(t, ws, entries)
	if r := Classify(ws); r.Class == ClassPhaseRefusal {
		t.Errorf("a refusal the cycle recovered from did not stop it: %+v", r)
	}
}

func TestClassify_CodedRefusalOutranksTheRegexPasses(t *testing.T) {
	ws := writeReport(t, "## Verdict\nAUDIT_FAIL: the auditor said no\n")
	writePhaseTiming(t, ws, refusalEntries(cyclestate.DiagCodeTriageProtectedSurface))
	if r := Classify(ws); r.Class != ClassPhaseRefusal {
		t.Errorf("the structured record outranks prose: %+v", r)
	}
}

func TestClassify_SentinelClassStillOutranksTheRefusal(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, refusalEntries(cyclestate.DiagCodeTriageProtectedSurface))
	// The sentinel comes from the phase that ended the cycle: its own classed FAIL is the authority.
	sentinel := "# triage\n<!-- evolve-verdict: {\"phase\":\"triage\",\"verdict\":\"FAIL\",\"failure\":{\"class\":\"infrastructure\"}} -->\n"
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := Classify(ws); r.Class == ClassPhaseRefusal {
		t.Errorf("pass 0 (the ending phase's own sentinel class) keeps winning: %+v", r)
	}
}

// A classed FAIL sentinel from an EARLIER phase (a scout that wedged, retried
// and passed) is stale once the record says the cycle stopped on a coded
// refusal — pass 0 is recency-aware (architecture review MEDIUM-2).
func TestClassify_StaleEarlierSentinelDoesNotOutrankTheRefusal(t *testing.T) {
	ws := t.TempDir()
	writePhaseTiming(t, ws, refusalEntries(cyclestate.DiagCodeTriageProtectedSurface))
	sentinel := "# scout\n<!-- evolve-verdict: {\"phase\":\"scout\",\"verdict\":\"FAIL\",\"failure\":{\"class\":\"infrastructure\"}} -->\n"
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := Classify(ws); r.Class != ClassPhaseRefusal {
		t.Errorf("the record's last outcome outranks an earlier phase's sentinel: %+v", r)
	}
}

func TestClassify_UnreadableRecord_FallsThrough(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(phasetiming.Path(ws), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := Classify(ws); r.Class == ClassPhaseRefusal {
		t.Errorf("a corrupt record is not a refusal: %+v", r)
	}
}
