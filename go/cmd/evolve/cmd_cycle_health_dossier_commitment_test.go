package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const dossierCommitmentSignal = "dossier_commitment"

// dossierFixture writes cycle n's dossier (nil tasks omits the field, as a
// legacy record does) and an empty workspace the CLI's fallback resolves back
// to root. Other signals fire on that empty workspace; they are not asserted.
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

// cycleHealthAnomalyLines runs the CLI and returns its dossier_commitment lines.
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
