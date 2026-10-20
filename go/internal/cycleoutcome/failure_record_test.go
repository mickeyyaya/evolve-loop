package cycleoutcome

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const laneBuildFloorReason = "review gate: phase \"build\" deliverable rejected after 2 correction(s): build handoff floor: ./acs/cycle7 (-tags acs): unit tests FAIL"

func sealLaneFail(t *testing.T, ws, report, class string) {
	t.Helper()
	sentinel := "<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\",\"schema_version\":2,\"failure\":{\"class\":\"" + class + "\"}} -->\n"
	if err := os.WriteFile(filepath.Join(ws, report), []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(ws, "audit-fail-reason.json"), map[string]any{"schema_version": 1, "phase": "build", "reasons": []string{laneBuildFloorReason}})
}

func closeOutSealedFail(t *testing.T, ceiling int, report, class string) (inbox string, res inboxmover.OutcomeResult) {
	t.Helper()
	root, inbox, ws := seedProject(t, "lane-item", []string{"lane-item"})
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "lane-item", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	sealLaneFail(t, ws, report, class)
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard)
	in.Ceiling = ceiling
	res, err := ApplyFailure(in)
	if err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	return inbox, res
}

func TestSealedLaneFail_RaisesFailureCountAndRecordsTheLadderClass(t *testing.T) {
	cases := map[string]struct {
		report, class string
		want          retryladder.Class
	}{
		"a build floor FAIL":       {"build-report.md", "code-build-fail", retryladder.Capability},
		"an audit FAIL":            {"audit-report.md", "code-audit-fail", retryladder.Capability},
		"an intent-malformed FAIL": {"build-report.md", "intent-malformed", retryladder.Spec},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			inbox, _ := closeOutSealedFail(t, 3, tc.report, tc.class)
			rec := itemRecord(t, inbox, "lane-item")
			if rec["failure_count"] != float64(1) {
				t.Fatalf("failure_count = %v, want 1: a sealed FAIL of the lane item must charge it, or the deep floor and the ceiling never fire: %v", rec["failure_count"], rec)
			}
			if rec["last_failure_class"] != string(tc.want) {
				t.Errorf("last_failure_class = %v, want %q", rec["last_failure_class"], tc.want)
			}
			if want := retryladder.Fingerprint(laneBuildFloorReason); rec["last_failure_fingerprint"] != want {
				t.Errorf("last_failure_fingerprint = %v, want %q", rec["last_failure_fingerprint"], want)
			}
		})
	}
}

func TestSealedLaneFail_AtTheCeilingParksTheItemInQuarantine(t *testing.T) {
	inbox, res := closeOutSealedFail(t, 1, "build-report.md", "code-build-fail")
	if hasItem(t, inbox, "lane-item") || len(res.Quarantined) != 1 {
		t.Fatalf("at ceiling 1 the task-level FAIL parks the item: root=%v res=%+v", hasItem(t, inbox, "lane-item"), res)
	}
	if rec := itemRecord(t, filepath.Join(inbox, "quarantine"), "lane-item"); rec["last_failure_class"] != string(retryladder.Capability) {
		t.Errorf("the parked item keeps its failure record: %v", rec)
	}
}

func TestSealedFail_InfraDoesNotRaiseFailureCount(t *testing.T) {
	cases := map[string]string{
		"an infrastructure FAIL": "infrastructure-transient",
		"an exit-transport hang": "exit-transport-hang",
	}
	for name, class := range cases {
		t.Run(name, func(t *testing.T) {
			inbox, _ := closeOutSealedFail(t, 1, "build-report.md", class)
			rec := itemRecord(t, inbox, "lane-item")
			if _, bumped := rec["failure_count"]; bumped {
				t.Errorf("an infra failure is not the item's fault: %v", rec)
			}
		})
	}
}

func TestFailureClassOf_MapsEveryCycleClassToALadderClass(t *testing.T) {
	cases := []struct {
		cls  cycleclassify.Result
		want retryladder.Class
	}{
		{cycleclassify.Result{Class: cycleclassify.ClassInfrastructure, Marker: cycleclassify.MarkerQuotaLikelyEmptyOutput}, retryladder.Quota},
		{cycleclassify.Result{Class: cycleclassify.ClassInfrastructure}, retryladder.Infra},
		{cycleclassify.Result{Class: "infrastructure-systemic"}, retryladder.Infra},
		{cycleclassify.Result{Class: cycleclassify.ClassExitTransportHang}, retryladder.Infra},
		{cycleclassify.Result{Class: cycleclassify.ClassBuildFail}, retryladder.Capability},
		{cycleclassify.Result{Class: "code-audit-fail"}, retryladder.Capability},
		{cycleclassify.Result{Class: "intent-rejected"}, retryladder.Spec},
		{cycleclassify.Result{Class: cycleclassify.ClassIntegrityBreach}, retryladder.Unknown},
	}
	for _, tc := range cases {
		if got := failureClassOf(tc.cls); got != tc.want {
			t.Errorf("failureClassOf(%+v) = %q, want %q", tc.cls, got, tc.want)
		}
	}
}

func TestIsTaskLevelFailure_ReadsTheSentinelVocabulary(t *testing.T) {
	for c, want := range map[cycleclassify.Classification]bool{
		"code-build-fail":          true,
		"code-audit-fail":          true,
		"intent-malformed":         true,
		"intent-rejected":          true,
		"infrastructure-transient": false,
		"code-audit-warn":          false,
		"build-fail":               true,
		"audit-fail":               true,
		"ship-gate-config":         true,
		"integrity-breach":         false,
		"infrastructure":           false,
	} {
		if got := IsTaskLevelFailure(c); got != want {
			t.Errorf("IsTaskLevelFailure(%q) = %v, want %v", c, got, want)
		}
	}
}

func TestFirstFailureReason_FailsOpenToEmpty(t *testing.T) {
	ws := t.TempDir()
	if got := firstFailureReason(ws); got != "" {
		t.Errorf("no file: %q", got)
	}
	writeJSON(t, filepath.Join(ws, "audit-fail-reason.json"), map[string]any{"reasons": []string{}})
	if got := firstFailureReason(ws); got != "" {
		t.Errorf("no reasons: %q", got)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := firstFailureReason(ws); got != "" {
		t.Errorf("bad json: %q", got)
	}
}
