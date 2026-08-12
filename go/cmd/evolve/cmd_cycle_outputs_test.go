package main

// cmd_cycle_outputs_test.go — `evolve cycle outputs [N]`: the operator's answer
// to "did every phase this cycle ran leave enough data to review?", plus the
// chain status with its one-meaning-per-state totalization. Driven through the
// real entry point with a real on-disk workspace, because the defect class here
// is a renderer that diverges from what the phases actually wrote.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cycleOutputsFixture(t *testing.T, cycle string, auditRan bool, chainPresent *bool) string {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-"+cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	completed := []string{"scout", "build"}
	if auditRan {
		completed = append(completed, "audit")
	}
	run, _ := json.Marshal(map[string]any{"cycle_id": cycle, "completed_phases": completed})
	if err := os.WriteFile(filepath.Join(ws, "run.json"), run, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, phase := range completed {
		report := phase + "-report.md"
		if phase == "audit" {
			report = "audit-report.md"
		}
		for _, f := range []string{report, phase + "-prompt.txt", phase + "-events.ndjson", phase + "-usage.json"} {
			if err := os.WriteFile(filepath.Join(ws, f), []byte("data\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// scout deliberately loses its events stream: the gap the report must name.
	if err := os.Remove(filepath.Join(ws, "scout-events.ndjson")); err != nil {
		t.Fatal(err)
	}
	if chainPresent != nil {
		rec, _ := json.Marshal(map[string]any{"chain_present": *chainPresent})
		if err := os.WriteFile(filepath.Join(ws, "audit-chain-shadow.json"), rec, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRunCycleOutputs_NamesEveryGapAndTheChainState(t *testing.T) {
	yes := true
	root := cycleOutputsFixture(t, "42", true, &yes)
	var out, errBuf strings.Builder
	if rc := runCycleOutputs([]string{"-project-root", root, "42"}, &out, &errBuf); rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{
		"2/3 complete",                       // scout gapped, build+audit complete
		"scout: scout-events.ndjson missing", // the gap, named
		"chain: chain-present",               // the totalized state
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
}

// The state that fixes the measurement flaw: an audit that never ran is
// reported as exactly that — never as non-compliance.
func TestRunCycleOutputs_AuditNotRunIsItsOwnState(t *testing.T) {
	root := cycleOutputsFixture(t, "43", false, nil)
	var out, errBuf strings.Builder
	if rc := runCycleOutputs([]string{"-project-root", root, "43"}, &out, &errBuf); rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, errBuf.String())
	}
	if !strings.Contains(out.String(), "chain: audit-not-run") {
		t.Errorf("an unaudited cycle must report audit-not-run, not absence:\n%s", out.String())
	}
}

func TestRunCycleOutputs_JSONCarriesTheSameFacts(t *testing.T) {
	yes := false
	root := cycleOutputsFixture(t, "44", true, &yes)
	var out, errBuf strings.Builder
	if rc := runCycleOutputs([]string{"-json", "-project-root", root, "44"}, &out, &errBuf); rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, errBuf.String())
	}
	var env struct {
		Chain string   `json:"chain"`
		Gaps  []string `json:"gaps"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("-json must decode: %v\n%s", err, out.String())
	}
	if env.Chain != "chain-absent" {
		t.Errorf("chain = %q, want chain-absent (record present, auditor did not comply)", env.Chain)
	}
	if len(env.Gaps) == 0 {
		t.Error("gaps must ride the envelope — the machine consumer cannot read prose")
	}
	// The envelope keys themselves are the contract: a rename silently blanks
	// whatever the operator's tooling reads, with no decode error to catch it.
	for _, key := range []string{`"cycle"`, `"chain"`, `"gaps"`, `"rows"`} {
		if !strings.Contains(out.String(), key) {
			t.Errorf("envelope lost wire key %s:\n%s", key, out.String())
		}
	}
}

// An existing-but-unparseable shadow record is a recorder defect, not a
// missing record — the review finding that classifying a truncated
// best-effort write as "record-missing" re-created the exact conflation the
// totalization was built to end.
func TestRunCycleOutputs_CorruptShadowRecordIsItsOwnState(t *testing.T) {
	root := cycleOutputsFixture(t, "45", true, nil)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-45")
	if err := os.WriteFile(filepath.Join(ws, "audit-chain-shadow.json"), []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf strings.Builder
	if rc := runCycleOutputs([]string{"-project-root", root, "45"}, &out, &errBuf); rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, errBuf.String())
	}
	if !strings.Contains(out.String(), "chain: record-corrupt") {
		t.Errorf("a truncated record must read record-corrupt, never record-missing:\n%s", out.String())
	}
}
