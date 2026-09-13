package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cmd_cycle_health_dossier_commitment_test.go — cycle 1652 RED contract, AC3 of
// triage-empty-commitment-still-dispatches-spine: "a cycle 1623-shaped dossier
// (empty top_n + full PhasesRun) … cyclehealth classifies such a historical
// dossier as an anomaly if it appears."
//
// Driven through the PRODUCTION caller (`evolve cycle-health <N> <workspace>`
// → runCycleHealth → cyclehealth.Check) rather than cyclehealth.Options
// directly, so the Builder is free to shape how the dossier reaches the check
// (an Options field threaded from projectRoot, or a derivation) while the
// contract stays: the CLI, given EVOLVE_PROJECT_ROOT, reads
// <root>/knowledge-base/cycles/cycle-N.json and reports the anomaly.
//
// Vocabulary pinned (the Builder implements it; the ACS predicates bind it):
//
//	signal   "dossier_commitment"
//	message  names at least one implementation phase the dossier recorded
//	          (tdd|build|audit|ship) so the operator sees WHAT ran against nothing
//
// The historical record is the DOSSIER, not the run dir — run dirs are
// gitignored and pruned, the dossier is committed — so the fixtures carry ONLY
// a dossier plus an empty workspace directory. Other signals (missing
// scout-report.md, …) will fire on that empty workspace; they are irrelevant
// here and deliberately not asserted on.

const dossierCommitmentSignal = "dossier_commitment"

// dossierFixture writes <root>/knowledge-base/cycles/cycle-<n>.json with the
// given committed task set (nil ⇒ the field is omitted: a pre-Tasks legacy
// record) and phase names, and <root>/.evolve/runs/cycle-<n>/ as the workspace
// the CLI's fallback root derivation (three Dir hops) resolves back to root.
func dossierFixture(t *testing.T, cycle int, tasks []string, phases []string) (root, workspace string) {
	t.Helper()
	root = t.TempDir()
	workspace = filepath.Join(root, ".evolve", "runs", "cycle-"+itoa(cycle))
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := map[string]any{
		"cycle":         cycle,
		"run_id":        "01JHISTORICALRUN000000000",
		"goal":          "historical record under test",
		"final_verdict": "PASS",
	}
	if tasks != nil {
		d["tasks"] = tasks
	}
	var recs []map[string]any
	for _, p := range phases {
		recs = append(recs, map[string]any{"name": p, "verdict": "PASS", "duration_ms": 1})
	}
	d["phases"] = recs
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-"+itoa(cycle)+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, workspace
}

func itoa(n int) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}

// cycleHealthAnomalyLines runs the CLI against the fixture and returns the
// stdout lines that carry the dossier_commitment signal.
func cycleHealthAnomalyLines(t *testing.T, root, workspace string, cycle int) (lines []string, stdout string, code int) {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	code = runCycleHealth([]string{itoa(cycle), workspace}, nil, &out, &errb)
	if code == 10 {
		t.Fatalf("cycle-health rejected its arguments (exit 10): %s", errb.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.Contains(line, dossierCommitmentSignal+":") {
			lines = append(lines, line)
		}
	}
	return lines, out.String(), code
}

var cycle1623Phases = []string{"scout", "triage", "tdd", "build", "audit", "tdd", "build", "audit", "tdd", "build", "audit", "ship"}

// TestCycleHealth_EmptyCommitmentDossierWithImplementationIsAnomaly — the
// defect shape: tasks [] and twelve phases. The report must carry the
// dossier_commitment signal and its message must name an implementation phase.
func TestCycleHealth_EmptyCommitmentDossierWithImplementationIsAnomaly(t *testing.T) {
	cases := []struct {
		name   string
		phases []string
	}{
		{"cycle-1623-twelve-phases", cycle1623Phases},
		{"single-build-after-empty-commitment", []string{"scout", "triage", "build"}},
		{"ship-only-after-empty-commitment", []string{"scout", "triage", "ship"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, ws := dossierFixture(t, 1623, []string{}, tc.phases)
			lines, stdout, _ := cycleHealthAnomalyLines(t, root, ws, 1623)
			if len(lines) == 0 {
				t.Fatalf("RED: no %q anomaly for an empty commitment that ran %v; cycle-health stdout:\n%s", dossierCommitmentSignal, tc.phases, stdout)
			}
			named := false
			for _, l := range lines {
				for _, impl := range []string{"tdd", "build", "audit", "ship"} {
					if strings.Contains(l, impl) {
						named = true
					}
				}
			}
			if !named {
				t.Errorf("RED: the %s anomaly must name the implementation phase(s) that ran; got %q", dossierCommitmentSignal, lines)
			}
			if !strings.Contains(stdout, dossierCommitmentSignal) {
				t.Errorf("RED: %q must be listed among the signals the CLI ran/reported", dossierCommitmentSignal)
			}
		})
	}
}

// TestCycleHealth_DossierCommitmentSignalStaysQuietOnHealthyShapes — the
// NEGATIVE / edge rows. A check that fires on every empty commitment, on every
// legacy record without the tasks field, or on a missing dossier would page
// the operator on healthy history.
func TestCycleHealth_DossierCommitmentSignalStaysQuietOnHealthyShapes(t *testing.T) {
	cases := []struct {
		name    string
		tasks   []string // nil ⇒ field omitted (legacy)
		phases  []string
		dossier bool
	}{
		{"empty-commitment-ended-at-triage", []string{}, []string{"scout", "triage"}, true},
		{"empty-commitment-resumed-at-triage", []string{}, []string{"triage"}, true},
		{"legacy-record-without-tasks-field", nil, cycle1623Phases, true},
		{"committed-work-with-full-spine", []string{"a-task"}, cycle1623Phases, true},
		{"empty-commitment-then-retro-only", []string{}, []string{"scout", "triage", "retro"}, true},
		{"no-dossier-on-disk", nil, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var root, ws string
			if tc.dossier {
				root, ws = dossierFixture(t, 1630, tc.tasks, tc.phases)
			} else {
				root = t.TempDir()
				ws = filepath.Join(root, ".evolve", "runs", "cycle-1630")
				if err := os.MkdirAll(ws, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			lines, _, _ := cycleHealthAnomalyLines(t, root, ws, 1630)
			if len(lines) != 0 {
				t.Errorf("%s anomaly raised on a healthy shape: %q", dossierCommitmentSignal, lines)
			}
		})
	}
}
