package convergence_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func TestLateBarFilesMediumBlocksHigh(t *testing.T) {
	for _, sev := range []convergence.Severity{convergence.SeverityMedium, convergence.SeverityLow} {
		d := afterOneFix(convergence.LoopAuditRepair, finding("x", sev, late))
		requireOutcome(t, d, convergence.ActionLand, 0)
		if !reflect.DeepEqual(d.File, []string{"x"}) {
			t.Fatalf("a late %s audit finding: file %q, want it filed", sev, d.File)
		}
	}
	for _, sev := range []convergence.Severity{convergence.SeverityHigh, convergence.SeverityCritical} {
		d := afterOneFix(convergence.LoopAuditRepair, finding("x", sev, late))
		requireOutcome(t, d, convergence.ActionContinue, 1)
		if len(d.File) != 0 {
			t.Fatalf("a late %s audit finding: file %q, want it blocking", sev, d.File)
		}
	}
}

func TestLateBarFilesARefutedLateCriticalOnlyForALoopWithABar(t *testing.T) {
	refutedCritical := finding("x", convergence.SeverityCritical, late, refuted)
	if d := afterOneFix(convergence.LoopAuditRepair, refutedCritical); !reflect.DeepEqual(d.File, []string{"x"}) {
		t.Fatalf("audit-repair: file %q, want the refuted late CRITICAL filed", d.File)
	}
	if d := afterOneFix(convergence.LoopCycle, refutedCritical); len(d.File) != 0 {
		t.Fatalf("a loop with no late bar filed %q", d.File)
	}
}

func TestLateBarDoesNotFileInfoOrCapabilityForAuditRepair(t *testing.T) {
	for _, f := range []convergence.Finding{
		finding("x", convergence.SeverityMedium, capability),
		finding("x", convergence.SeverityInfo),
	} {
		d := afterOneFix(convergence.LoopAuditRepair, f)
		if len(d.File) != 0 {
			t.Fatalf("audit finding %+v filed %q", f, d.File)
		}
	}
}

func TestLateFiled_SharesTheRuleTableWithDecide(t *testing.T) {
	for _, tc := range []struct {
		loop convergence.Loop
		f    convergence.Finding
		want bool
	}{
		{convergence.LoopAuditRepair, finding("x", convergence.SeverityMedium, late), true},
		{convergence.LoopAuditRepair, finding("x", convergence.SeverityMedium), false},
		{convergence.LoopAuditRepair, finding("x", convergence.SeverityHigh, late), false},
		{convergence.LoopAuditRepair, finding("x", convergence.SeverityHigh, late, refuted), true},
		{convergence.LoopCodeReview, finding("x", convergence.SeverityLow, late), true},
		{convergence.LoopAuditRepair, finding("x", convergence.SeverityLow, late, fixed), false},
	} {
		if got := convergence.LateFiled(tc.loop, tc.f); got != tc.want {
			t.Errorf("LateFiled(%s, %+v) = %v, want %v", tc.loop, tc.f, got, tc.want)
		}
	}
}

func TestLateFileBar_IsTheSeverityWhereLateFiledFlips(t *testing.T) {
	for _, loop := range []convergence.Loop{convergence.LoopAuditRepair, convergence.LoopCodeReview, convergence.LoopConsoleLane} {
		bar := convergence.LateFileBar(loop)
		if bar == "" {
			t.Fatalf("loop %s has no late bar", loop)
		}
		if convergence.LateFiled(loop, finding("x", bar, late)) {
			t.Errorf("loop %s files a late %s finding at its own bar", loop, bar)
		}
		if !convergence.LateFiled(loop, finding("x", convergence.SeverityLow, late)) {
			t.Errorf("loop %s does not file a late LOW finding below its bar %s", loop, bar)
		}
	}
	if bar := convergence.LateFileBar(convergence.LoopCycle); bar != "" {
		t.Errorf("a loop with no late bar reports %q", bar)
	}
}

func TestInsideHunks_ChecksFileAndLineRange(t *testing.T) {
	hunks := []convergence.Hunk{{File: "a/b.go", From: 10, To: 20}}
	for loc, want := range map[string]bool{"a/b.go:10": true, "a/b.go:20": true, "a/b.go:21": false, "a/c.go:15": false, "a/b.go": false, "": false} {
		if got := convergence.InsideHunks(loc, hunks); got != want {
			t.Errorf("InsideHunks(%q) = %v, want %v", loc, got, want)
		}
	}
}
