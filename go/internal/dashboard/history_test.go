package dashboard

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// writeDossier commits a minimal knowledge-base/cycles/cycle-N.json the way
// the dossier producer does (dossier.RenderJSON), so the history reader is
// exercised against the real schema.
func writeDossier(t *testing.T, root string, d dossier.Dossier) {
	t.Helper()
	buf, err := dossier.RenderJSON(&d)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "knowledge-base", "cycles")
	writeFile(t, filepath.Join(dir, dossierFileName(d.Cycle)), string(buf))
}

func failDossier(cycle int, fp string) dossier.Dossier {
	return dossier.Dossier{Cycle: cycle, Goal: "g", FinalVerdict: dossier.VerdictFail,
		Phases:    []dossier.PhaseRecord{{Name: "audit", Verdict: "FAIL"}, {Name: "audit", Verdict: "FAIL"}},
		Defects:   []dossier.Defect{{ID: "audit-fail", Severity: "HIGH", Summary: "cycle did not pass audit"}},
		Carryover: []dossier.Carryover{{ID: "address-audit-findings", Action: "address the audit findings"}},
		Failure:   &dossier.FailureRecord{Fingerprint: fp, PreClass: "gate-block", Reasons: []string{"EGPS ship_eligible=false"}}}
}

func passDossier(cycle int) dossier.Dossier {
	return dossier.Dossier{Cycle: cycle, Goal: "g", FinalVerdict: dossier.VerdictPass, CommitSHA: "abc123",
		Phases: []dossier.PhaseRecord{{Name: "audit", Verdict: "PASS"}, {Name: "ship", Verdict: "PASS"}}}
}

func TestReadHistory_ShipRateAndFingerprints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// 1: FAIL fp-a · 2: PASS · 3: FAIL fp-a (regressed: a PASS sits between) · 4: FAIL fp-b · 5: PASS
	writeDossier(t, root, failDossier(1, "audit|gate-block|aaaa"))
	writeDossier(t, root, passDossier(2))
	writeDossier(t, root, failDossier(3, "audit|gate-block|aaaa"))
	writeDossier(t, root, failDossier(4, "audit|verdict-fail|bbbb"))
	writeDossier(t, root, passDossier(5))

	h := readHistory(root, newDossierCache())
	if h.Trend.Closed != 5 || h.Trend.Shipped != 2 {
		t.Fatalf("closed/shipped = %d/%d, want 5/2", h.Trend.Closed, h.Trend.Shipped)
	}
	if got := h.Trend.ShipRateAll; got < 0.39 || got > 0.41 {
		t.Fatalf("ShipRateAll = %v, want 0.4", got)
	}
	if len(h.Trend.Points) != 5 || h.Trend.Points[0].Cycle != 1 || h.Trend.Points[4].Cycle != 5 || !h.Trend.Points[4].Shipped {
		t.Fatalf("Points = %+v, want oldest-first 1..5 with 5 shipped", h.Trend.Points)
	}
	if len(h.Fingerprints) != 2 {
		t.Fatalf("Fingerprints = %+v, want 2 groups", h.Fingerprints)
	}
	// Most recent first: fp-b (last 4) then fp-a (last 3).
	if h.Fingerprints[0].Fingerprint != "audit|verdict-fail|bbbb" || h.Fingerprints[0].Count != 1 {
		t.Fatalf("Fingerprints[0] = %+v", h.Fingerprints[0])
	}
	a := h.Fingerprints[1]
	if a.Count != 2 || a.FirstCycle != 1 || a.LastCycle != 3 || !a.Regressed || a.Reason == "" {
		t.Fatalf("fp-a stat = %+v, want count 2, first 1, last 3, regressed, reason", a)
	}
	if h.Fingerprints[0].Regressed {
		t.Fatalf("fp-b must not be regressed (single occurrence)")
	}
	if len(h.Dossiers) != 5 || h.Dossiers[3].Failure == nil {
		t.Fatalf("Dossiers map = %d entries", len(h.Dossiers))
	}
}

func TestReadHistory_MissingDirIsEmptyNotError(t *testing.T) {
	t.Parallel()
	h := readHistory(t.TempDir(), newDossierCache())
	if h.Trend.Closed != 0 || len(h.Fingerprints) != 0 || len(h.Warnings) != 0 {
		t.Fatalf("empty root: %+v", h)
	}
}

func TestReadHistory_MalformedDossierIsWarnedNotFatal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDossier(t, root, passDossier(1))
	dir := filepath.Join(root, "knowledge-base", "cycles")
	writeFile(t, filepath.Join(dir, "cycle-2.json"), "{not json")
	h := readHistory(root, newDossierCache())
	if h.Trend.Closed != 1 || len(h.Warnings) != 1 {
		t.Fatalf("closed=%d warnings=%v, want 1 closed + 1 warning", h.Trend.Closed, h.Warnings)
	}
}

func TestDossierCache_ReusesUnchangedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDossier(t, root, passDossier(7))
	c := newDossierCache()
	readHistory(root, c)
	first := c.parses
	readHistory(root, c)
	if c.parses != first {
		t.Fatalf("second read re-parsed an unchanged dossier: parses %d -> %d", first, c.parses)
	}
	// A rewrite (new mtime+size) is picked up.
	writeDossier(t, root, failDossier(7, "z"))
	h := readHistory(root, c)
	if h.Trend.Shipped != 0 {
		t.Fatalf("cache served the stale PASS after the file changed")
	}
}
