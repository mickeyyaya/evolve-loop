//go:build acs

// Package cycle1301 materialises the cycle-1301 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	T1 demotion-ledger-remedy-status-field
//	(todo-id demotion-ledger-records-salvage-attempted-vs-no-remedy-possible)
//
// What the cycle must deliver. The demotion ledger record — the JSON the
// triage capacity clamp auto-files at .evolve/inbox/auto-heuristic-demotion-
// triagecap-c<older>-c<newer>.json when the identical-rejection pattern fires
// (ADR-0046 Layer 2, go/internal/triagecap/demotion.go) — today carries only a
// prose `action` narrative. Nothing on the record says whether a salvage of the
// underlying gate defect was ATTEMPTED or whether the loop concluded NO REMEDY
// was possible; commit 29915424 had to explain two gate demotions in a queue
// chore commit body because the ledger itself cannot answer that. This cycle
// adds an explicit, caller-declared `remedy_status` field with a closed
// vocabulary {pending, salvage_attempted, no_remedy_possible} and an honest
// default (`pending` at file time — the demotion just fired, no remedy decision
// has been made yet), never a silent blank and never an unvalidated string.
//
// Predicate strategy — every predicate exercises real behaviour, never a source
// grep (the cycle-85 degenerate-predicate ban):
//
//   - 001 is the WIRING PROOF and the crux: it drives the REAL production
//     caller — triagecap.NewReviewer(config.StageEnforce).Review(...) over a
//     temp project whose state.json replays the 301/302 identical-rejection
//     pair — and asserts the ledger file the reviewer itself wrote carries
//     remedy_status=="pending". A predicate that called the record builder
//     directly would pass on dead code; this one stays RED until reviewer.go's
//     call site actually threads the field through.
//   - 002 is the regression guard: the SAME reviewer-written file must keep
//     every pre-existing field (id, action, priority, weight, relieved_cycle,
//     evidence_pointer, injected_by) at its current shape and value. An
//     additive field must not disturb what the ledger already promised.
//   - 003 pins the closed vocabulary and the normalisation contract through the
//     exported seam triagecap.NormalizeRemedyStatus: the three canonical values
//     survive verbatim; blank, unknown, wrong-case and whitespace-padded input
//     fall back to pending and are NEVER echoed verbatim (the negative case).
//   - 004 pins that the record BUILDER honours an explicitly declared terminal
//     outcome: a record built with salvage_attempted / no_remedy_possible
//     marshals those exact strings into the `remedy_status` JSON key, and a
//     junk status is normalised rather than written through.
//
// Fixture note: predicates 001/002 deliberately use the package's REAL seams
// (KnownPackages / readWindow / readFailedApproaches over a temp project root)
// rather than the in-package unexported test hooks — the external package can
// only reach the production constructor, which is exactly the reachability
// property being proven. The temp root holds no Go packages, so floor counting
// falls back to the min-1 prose rule: three floor-bearing ## top_n items count
// 3 against a cap of 2 (window K=1 ⇒ Cap=ceil(1.25)=2), which puts Review on
// its rejection path and therefore into the demotion consult.
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

// Verbatim rejection summaries from state.json:failedApproaches, cycles
// 301/302 — the same corpus demotion_test.go replays. Both carry the gate
// marker ("triage overpacked") and the phase marker ("during triage:"), and
// differ only by same-magnitude jitter (6 vs 7 floors), so they collapse to one
// reason template and the demotion fires.
const (
	summary301 = `cycle 301 failed during triage: review gate: phase "triage" deliverable rejected after 2 correction(s): triage overpacked: 6 committed coverage floors exceed the capacity cap 5 (= ceil(1.25×K), K=4 observed floors/turn over 1 shipped cycles). Re-emit the triage report keeping at most 5 coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.`
	summary302 = `cycle 302 failed during triage: review gate: phase "triage" deliverable rejected after 2 correction(s): triage overpacked: 7 committed coverage floors exceed the capacity cap 5 (= ceil(1.25×K), K=4 observed floors/turn over 1 shipped cycles). Re-emit the triage report keeping at most 5 coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.`
)

// overpackedArtifact is a ## top_n whose three floor-bearing items each carry a
// ≥-marked target percent, so each counts one floor under the min-1 aggregate
// rule (no known packages resolve in a temp project root).
const overpackedArtifact = `## top_n (commit to THIS cycle)
- coverage-a: push swarmrunner coverage to ≥98%
- coverage-b: push bridge coverage to ≥98%
- coverage-c: push evalgate coverage to ≥98%

## deferred (carry to NEXT cycle's carryoverTodos)
`

// demotedLedgerRecord drives the real production reviewer through its demotion
// path and returns the parsed ledger JSON the reviewer itself auto-filed. Any
// deviation (reviewer approved without demoting, no file written, more than one
// file) fails here rather than surfacing as a confusing field assertion.
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

// TestC1301_001_LedgerRecordCarriesPendingRemedyStatus is the wiring proof: the
// field must be written by the PRODUCTION path (reviewer.go's demotion call
// site), and its file-time value must be the honest default `pending` — never
// absent, never blank. AC1 + AC2.
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

// TestC1301_002_LedgerRecordPreservesExistingFields is the regression guard: the
// new field is ADDITIVE. Every field the ledger already promised keeps its
// shape and value through the same production write. AC3.
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

// TestC1301_003_RemedyStatusVocabularyIsClosed pins the closed vocabulary and
// the normalisation contract. The negative half is the load-bearing one: an
// unknown or malformed status must NEVER be written through verbatim — it falls
// back to pending, so the ledger can only ever say one of three things. AC1 +
// AC4 (negative / edge / OOD).
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

// TestC1301_004_ExplicitTerminalOutcomesReachTheRecord pins the whole point of
// the field: the two TERMINAL outcomes a caller may declare — salvage attempted
// vs no remedy possible — must reach the ledger's wire form exactly, through
// the same builder the production writer uses, while junk is normalised. AC1 +
// AC4 (semantic diversity: three distinct declared outcomes, one rejection).
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
			// The declared outcome must not cost the record its identity or
			// its relief bookkeeping.
			if got := decoded["id"]; got != "auto-heuristic-demotion-triagecap-c301-c302" {
				t.Errorf("id = %v, want the pair-identity slug", got)
			}
			if relieved, ok := decoded["relieved_cycle"].(float64); !ok || int(relieved) != 303 {
				t.Errorf("relieved_cycle = %v, want 303", decoded["relieved_cycle"])
			}
		})
	}
}
