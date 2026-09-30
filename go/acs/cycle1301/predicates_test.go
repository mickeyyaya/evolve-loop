//go:build acs

package cycle1301

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

const (
	summary301 = `cycle 301 failed during triage: review gate: phase "triage" deliverable rejected after 2 correction(s): triage overpacked: 6 committed coverage floors exceed the capacity cap 5 (= ceil(1.25×K), K=4 observed floors/turn over 1 shipped cycles). Re-emit the triage report keeping at most 5 coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.`
	summary302 = `cycle 302 failed during triage: review gate: phase "triage" deliverable rejected after 2 correction(s): triage overpacked: 7 committed coverage floors exceed the capacity cap 5 (= ceil(1.25×K), K=4 observed floors/turn over 1 shipped cycles). Re-emit the triage report keeping at most 5 coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.`
)

const overpackedArtifact = `## top_n (commit to THIS cycle)
- coverage-a: push swarmrunner coverage to ≥98%
- coverage-b: push bridge coverage to ≥98%
- coverage-c: push evalgate coverage to ≥98%

## deferred (carry to NEXT cycle's carryoverTodos)
`

func demotedLedgerRecord(t *testing.T) map[string]any {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-303")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"triageThroughput": []map[string]int{{"cycle": 300, "floors": 1}},
		"failedApproaches": []map[string]any{
			{"cycle": 301, "summary": summary301},
			{"cycle": 302, "summary": summary302},
		},
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "state.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, triagecap.TriageArtifactName()), []byte(overpackedArtifact), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "run.json"), []byte(`{"cycle_id":303}`), 0o644); err != nil {
		t.Fatal(err)
	}

	res := triagecap.NewReviewer(config.StageEnforce).Review(context.Background(),
		core.ReviewInput{Phase: "triage", Workspace: ws, ProjectRoot: root})
	if !res.Approve {
		t.Fatalf("the 301/302 identical-rejection pair must demote the clamp to shadow for cycle 303 (ADR-0046 L2); reviewer rejected instead: %s", res.Reason)
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "auto-heuristic-demotion-*.json"))
	if len(matches) != 1 {
		t.Fatalf("demotion must auto-file exactly one ledger record, found %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("reading the auto-filed ledger record: %v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("the ledger record must be valid JSON: %v (raw: %s)", err, data)
	}
	return rec
}

func TestC1301_001_LedgerRecordCarriesPendingRemedyStatus(t *testing.T) {
	rec := demotedLedgerRecord(t)

	got, ok := rec["remedy_status"]
	if !ok {
		keys := make([]string, 0, len(rec))
		for k := range rec {
			keys = append(keys, k)
		}
		t.Fatalf("the demotion ledger record must carry a remedy_status field so salvage-attempted vs no-remedy-possible is readable off the record itself; keys present: %v", keys)
	}
	s, isString := got.(string)
	if !isString {
		t.Fatalf("remedy_status must be a string, got %T (%v)", got, got)
	}
	if s != "pending" {
		t.Errorf("at file time no remedy decision has been made yet, so remedy_status must default to %q, got %q", "pending", s)
	}
}

func TestC1301_002_LedgerRecordPreservesExistingFields(t *testing.T) {
	rec := demotedLedgerRecord(t)

	if got := rec["id"]; got != "auto-heuristic-demotion-triagecap-c301-c302" {
		t.Errorf("id = %v, want the pair-identity slug auto-heuristic-demotion-triagecap-c301-c302", got)
	}
	if got := rec["priority"]; got != "HIGH" {
		t.Errorf("priority = %v, want HIGH", got)
	}
	weight, ok := rec["weight"].(float64)
	if !ok || weight != 0.7 {
		t.Errorf("weight = %v (%T), want 0.7", rec["weight"], rec["weight"])
	}
	relieved, ok := rec["relieved_cycle"].(float64)
	if !ok || int(relieved) != 303 {
		t.Errorf("relieved_cycle = %v (%T), want 303 — the cycle that consumed the pair's one-cycle relief", rec["relieved_cycle"], rec["relieved_cycle"])
	}
	action, _ := rec["action"].(string)
	if !strings.Contains(action, "byte-identical reason template") || !strings.Contains(action, "SHADOW for cycle 303") {
		t.Errorf("action narrative lost its ADR-0046 L2 explanation or its demoted-cycle statement: %q", action)
	}
	pointer, _ := rec["evidence_pointer"].(string)
	if !strings.Contains(pointer, "cycle-301") || !strings.Contains(pointer, "cycle-302") {
		t.Errorf("evidence_pointer = %q, want it to name both evidence cycles", pointer)
	}
	if got := rec["injected_by"]; got != "triagecap-demotion" {
		t.Errorf("injected_by = %v, want triagecap-demotion", got)
	}
	if s, _ := rec["injected_at"].(string); s == "" {
		t.Error("injected_at must stay populated")
	}
}

func TestC1301_003_RemedyStatusVocabularyIsClosed(t *testing.T) {
	if triagecap.RemedyPending != "pending" ||
		triagecap.RemedySalvageAttempted != "salvage_attempted" ||
		triagecap.RemedyNoRemedyPossible != "no_remedy_possible" {
		t.Fatalf("the ledger vocabulary is a wire contract: got (%q, %q, %q), want (pending, salvage_attempted, no_remedy_possible)",
			triagecap.RemedyPending, triagecap.RemedySalvageAttempted, triagecap.RemedyNoRemedyPossible)
	}

	cases := []struct {
		name string
		in   string
		want triagecap.RemedyStatus
	}{
		{"canonical pending survives", "pending", triagecap.RemedyPending},
		{"canonical salvage_attempted survives", "salvage_attempted", triagecap.RemedySalvageAttempted},
		{"canonical no_remedy_possible survives", "no_remedy_possible", triagecap.RemedyNoRemedyPossible},
		{"empty is not a silent blank", "", triagecap.RemedyPending},
		{"unknown value is rejected, not echoed", "salvaged-ish", triagecap.RemedyPending},
		{"wrong case is not a canonical value", "SALVAGE_ATTEMPTED", triagecap.RemedyPending},
		{"padded value is not a canonical value", " salvage_attempted ", triagecap.RemedyPending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := triagecap.NormalizeRemedyStatus(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeRemedyStatus(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if string(got) == "" {
				t.Errorf("NormalizeRemedyStatus(%q) returned a blank status — the ledger must never carry an empty remedy_status", tc.in)
			}
		})
	}
}

func TestC1301_004_ExplicitTerminalOutcomesReachTheRecord(t *testing.T) {
	const detail = "identical rejection template in cycles 301 and 302 (hash deadbeefdeadbeef)"

	cases := []struct {
		name string
		in   triagecap.RemedyStatus
		want string
	}{
		{"salvage attempted", triagecap.RemedySalvageAttempted, "salvage_attempted"},
		{"no remedy possible", triagecap.RemedyNoRemedyPossible, "no_remedy_possible"},
		{"pending", triagecap.RemedyPending, "pending"},
		{"junk is normalised, not written through", triagecap.RemedyStatus("who-knows"), "pending"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := triagecap.NewDemotionLedgerRecord(303, 301, 302, detail, tc.in)
			raw, err := json.Marshal(rec)
			if err != nil {
				t.Fatalf("the ledger record must marshal: %v", err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("marshalled ledger record must be valid JSON: %v", err)
			}
			if got := decoded["remedy_status"]; got != tc.want {
				t.Errorf("remedy_status = %v, want %q (declared %q)", got, tc.want, tc.in)
			}
			if got := decoded["id"]; got != "auto-heuristic-demotion-triagecap-c301-c302" {
				t.Errorf("id = %v, want the pair-identity slug", got)
			}
			if relieved, ok := decoded["relieved_cycle"].(float64); !ok || int(relieved) != 303 {
				t.Errorf("relieved_cycle = %v, want 303", decoded["relieved_cycle"])
			}
		})
	}
}
