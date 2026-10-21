package triagedecision

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const reportWithAcceptanceDeferral = "## top_n\n- item-a: fix it — files=a.go\n\n## deferred\n- item-z: later — defer_reason=next cycle\n\n## deferred_acceptance\n- item-a: acceptance 2 — defer_reason=needs the protected writer, console work\n- item-a: the second one — defer_reason=no index\n- item-b: acceptance 0 — defer_reason=zero\n- item-c: acceptance 3\n- item-q: acceptance 1 — defer_reason=item-q is not committed\n"

func TestDeferredAcceptance_ReadsEachCriterionTheReportDefers(t *testing.T) {
	t.Parallel()

	got := DeferredAcceptance(reportWithAcceptanceDeferral)

	want := []AcceptanceDeferral{{ID: "item-a", Index: 2, Reason: "needs the protected writer, console work"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DeferredAcceptance = %+v, want %+v: only a card for a top_n item, with a 1-based index and a reason, defers a criterion", got, want)
	}
}

func TestBuild_AnAcceptanceDeferralIsNotADeferredItem(t *testing.T) {
	t.Parallel()

	d, err := build(reportWithAcceptanceDeferral, 7, true)

	if err != nil || len(d.Deferred) != 1 || d.Deferred[0].ID != "item-z" || len(d.TopN) != 1 || d.TopN[0].ID != "item-a" {
		t.Errorf("decision = %+v, err = %v; want item-a committed and only item-z deferred", d, err)
	}
}

func TestDeferredAcceptance_AReportWithoutTheSectionDefersNothing(t *testing.T) {
	t.Parallel()
	if got := DeferredAcceptance("## top_n\n- item-a: fix it\n\n## deferred\n- item-z: later — defer_reason=next cycle\n"); got != nil {
		t.Errorf("DeferredAcceptance = %+v, want nil: a deferred item is not a deferred criterion", got)
	}
}

func TestDeferredAcceptanceIn_ReadsTheWorkspacesTriageReport(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(reportWithAcceptanceDeferral), 0o644); err != nil {
		t.Fatal(err)
	}

	got, none := DeferredAcceptanceIn(ws), DeferredAcceptanceIn(t.TempDir())

	if len(got) != 1 || got[0].ID != "item-a" || got[0].Index != 2 || none != nil {
		t.Errorf("DeferredAcceptanceIn = %+v and %+v without a report; want the one deferral and nil", got, none)
	}
}
