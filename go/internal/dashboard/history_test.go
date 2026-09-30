package dashboard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

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
	writeDossier(t, root, failDossier(7, "z"))
	h := readHistory(root, c)
	if h.Trend.Shipped != 0 {
		t.Fatalf("cache served the stale PASS after the file changed")
	}
}

func TestComputeTrend_ShipStreakCountsBackFromTheNewestCycleOverTheFullHistory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		oldest     string
		wantStreak int
		wantRun    *ZeroShipRun
	}{
		{"empty", "", 0, nil},
		{"all-shipped", "PPP", 3, nil},
		{"newest-unshipped", "PPF", 0, &ZeroShipRun{FirstCycle: 3, LastCycle: 3, Length: 1}},
		{"most-recent-run-only", "FPFFP", 1, &ZeroShipRun{FirstCycle: 3, LastCycle: 4, Length: 2}},
		{"warn-without-commit-breaks", "PwP", 1, &ZeroShipRun{FirstCycle: 2, LastCycle: 2, Length: 1}},
		{"beyond-the-point-cap", "F" + strings.Repeat("P", trendPointCap+5), trendPointCap + 5, &ZeroShipRun{FirstCycle: 1, LastCycle: 1, Length: 1}},
	}
	for _, c := range cases {
		ds := map[int]*dossier.Dossier{}
		for i, o := range c.oldest {
			d := &dossier.Dossier{Cycle: i + 1, FinalVerdict: dossier.VerdictFail}
			switch o {
			case 'P':
				d.FinalVerdict, d.CommitSHA = dossier.VerdictPass, "abc"
			case 'w':
				d.FinalVerdict = dossier.VerdictWarn
			}
			ds[i+1] = d
		}
		got := computeTrend(sortedCycles(ds), ds)
		if got.ShipStreak != c.wantStreak {
			t.Errorf("%s: ShipStreak=%d want %d", c.name, got.ShipStreak, c.wantStreak)
		}
		switch {
		case c.wantRun == nil && got.LastZeroShipRun != nil:
			t.Errorf("%s: LastZeroShipRun=%+v want nil", c.name, *got.LastZeroShipRun)
		case c.wantRun != nil && (got.LastZeroShipRun == nil || *got.LastZeroShipRun != *c.wantRun):
			t.Errorf("%s: LastZeroShipRun=%v want %+v", c.name, got.LastZeroShipRun, *c.wantRun)
		}
	}
}
