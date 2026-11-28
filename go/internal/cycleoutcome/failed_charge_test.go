package cycleoutcome

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func TestApplyFailure_ChargesOnlyThePinMembersTheDecisionNames(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		pin      []string
		decision string
		want     map[string]int
	}{
		{"an unmentioned menu mate is not charged", []string{"a", "b"}, `{"top_n":[{"id":"a"}]}`, map[string]int{"a": 1, "b": 0}},
		{"a deferred pin is owed and charged", []string{"a", "b"}, `{"top_n":[],"deferred":[{"id":"a"}]}`, map[string]int{"a": 1, "b": 1}},
		{"a deferred mate stays uncharged beside a named one", []string{"a", "b"}, `{"top_n":[{"id":"a"}],"deferred":[{"id":"b"}]}`, map[string]int{"a": 1, "b": 0}},
		{"a lone deferred pin is charged", []string{"a"}, `{"top_n":[],"deferred":[{"id":"a"}]}`, map[string]int{"a": 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			inbox := filepath.Join(root, ".evolve", "inbox")
			for _, id := range tc.pin {
				writeLaneItem(t, inbox, id, "go/internal/"+id+"/"+id+".go")
			}
			ws := filepath.Join(root, ".evolve", "runs", "cycle-12")
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := committedset.WriteRecord(root, committedset.Record{Cycle: 12, ItemIDs: tc.pin}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(tc.decision), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := ApplyFailure(FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 12, Ceiling: 3, Stderr: io.Discard}); err != nil {
				t.Fatalf("ApplyFailure: %v", err)
			}
			for id, want := range tc.want {
				if got := itemFailureCount(t, findItemFile(t, inbox, id)); got != want {
					t.Errorf("item %s failure_count = %d, want %d", id, got, want)
				}
			}
		})
	}
}

func TestIsTaskLevelResult_PreBuildAndHostFault(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cls  cycleclassify.Result
		want bool
	}{
		{"scout coded refusal", cycleclassify.Result{Class: cycleclassify.ClassPhaseRefusal, Marker: "SCOUT_REPORT_REJECTED", Phase: "scout"}, true},
		{"triage coded refusal", cycleclassify.Result{Class: cycleclassify.ClassPhaseRefusal, Marker: "TRIAGE_BRIEF_REJECTED", Phase: "triage"}, true},
		{"triage host fault", cycleclassify.Result{Class: cycleclassify.ClassPhaseRefusal, Marker: cyclestate.DiagCodeTriageLanePinMissing, Phase: "triage"}, false},
		{"triage commitment host fault", cycleclassify.Result{Class: cycleclassify.ClassPhaseRefusal, Marker: cyclestate.DiagCodeTriageCommitmentInvalid, Phase: "triage"}, false},
		{"audit coded refusal", cycleclassify.Result{Class: cycleclassify.ClassPhaseRefusal, Marker: "AUDIT_BRIEF_REJECTED", Phase: "audit"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsTaskLevelResult(tc.cls); got != tc.want {
				t.Errorf("IsTaskLevelResult(%+v) = %v, want %v", tc.cls, got, tc.want)
			}
		})
	}
}

func TestApplyFailure_PreBuildChargesAndHostFaultExempt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		entries  []phasetiming.Entry
		sentinel string
		want     int
	}{
		{
			name:    "real scout failure charges pin",
			entries: []phasetiming.Entry{{Phase: "scout", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Code: "SCOUT_REPORT_REJECTED"}}}},
			want:    1,
		},
		{
			name:    "host fault lane pin missing charges nothing",
			entries: []phasetiming.Entry{{Phase: "scout", Verdict: "PASS"}, {Phase: "triage", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Code: cyclestate.DiagCodeTriageLanePinMissing}}}},
			want:    0,
		},
		{
			name:     "infrastructure scout failure charges nothing",
			entries:  []phasetiming.Entry{{Phase: "scout", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Code: "SCOUT_REPORT_REJECTED"}}}},
			sentinel: "<!-- evolve-verdict: {\"phase\":\"scout\",\"verdict\":\"FAIL\",\"failure\":{\"class\":\"infrastructure\"}} -->\n",
			want:     0,
		},
		{
			name:    "post-build audit failure stays uncharged",
			entries: []phasetiming.Entry{{Phase: "scout", Verdict: "PASS"}, {Phase: "build", Verdict: "PASS"}, {Phase: "audit", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Code: "AUDIT_BRIEF_REJECTED"}}}},
			want:    0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			inbox := filepath.Join(root, ".evolve", "inbox")
			writeLaneItem(t, inbox, "pin-a", "go/internal/a/a.go")
			ws := filepath.Join(root, ".evolve", "runs", "cycle-31")
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := committedset.WriteRecord(root, committedset.Record{Cycle: 31, ItemIDs: []string{"pin-a"}}); err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(tc.entries)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(phasetiming.Path(ws), raw, 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.sentinel != "" {
				if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte(tc.sentinel), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 31, io.Discard)
			if _, err := ApplyFailure(in); err != nil {
				t.Fatalf("ApplyFailure: %v", err)
			}
			if got := itemFailureCount(t, findItemFile(t, inbox, "pin-a")); got != tc.want {
				t.Errorf("failure_count = %d, want %d", got, tc.want)
			}
		})
	}
}
