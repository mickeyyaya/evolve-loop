// verify_unit_test.go — seam-injected unit tests for the ship-class
// verification paths the integration matrix skips:
//
//   - checkEGPSGate     (audit.go) — the trust-kernel RED gate
//   - verifyTrivial     (verify.go) — --class trivial audit-bypass guard
//   - verifyManualConfirm (verify.go) — --class manual auto-confirm path
//
// These pin EXISTING safety-critical contracts. The most load-bearing is
// the EGPS gate's "red_count != 0 ⇒ refuse ship" branch (the v10.0.0
// trust-kernel invariant): a silent regression there would let a build
// with RED predicates ship. See docs/architecture/egps-v10.md.
package ship

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// --- checkEGPSGate -------------------------------------------------------

// TestCheckEGPSGate_MissingFile is the pre-v10.0.0 bootstrap path: no
// acs-verdict.json yet ⇒ gate is a no-op (fluent posture from the audit
// report still applies). Must NOT error.
func TestCheckEGPSGate_MissingFile(t *testing.T) {
	res := &RunResult{}
	if err := checkEGPSGate(filepath.Join(t.TempDir(), "acs-verdict.json"), res); err != nil {
		t.Fatalf("missing file must be a soft no-op; got %v", err)
	}
	if len(res.Logs) != 0 {
		t.Errorf("missing file must not append logs; got %v", res.Logs)
	}
}

// TestCheckEGPSGate_RedCountZero is the clean-ship path: red_count==0
// returns nil and appends a confirmation log line.
func TestCheckEGPSGate_RedCountZero(t *testing.T) {
	path := writeACSVerdict(t, map[string]any{
		"red_count":       0,
		"green_count":     12,
		"verdict":         "PASS",
		"predicate_suite": map[string]any{"total": 12},
	})
	res := &RunResult{}
	if err := checkEGPSGate(path, res); err != nil {
		t.Fatalf("red_count==0 must pass; got %v", err)
	}
	if len(res.Logs) == 0 || !strings.Contains(res.Logs[len(res.Logs)-1], "EGPS predicate suite verdict=PASS") {
		t.Errorf("expected EGPS-OK log line; got %v", res.Logs)
	}
}

// TestCheckEGPSGate_RedCountNonZero is THE trust-kernel invariant: any
// RED predicate must produce an IntegrityError that blocks the ship and
// names the offending predicate IDs. This is the highest-value assertion
// in this file.
func TestCheckEGPSGate_RedCountNonZero(t *testing.T) {
	path := writeACSVerdict(t, map[string]any{
		"red_count":       2,
		"green_count":     5,
		"verdict":         "FAIL",
		"red_ids":         []string{"pred-auth-leak", "pred-null-deref"},
		"predicate_suite": map[string]any{"total": 7},
	})
	res := &RunResult{}
	err := checkEGPSGate(path, res)
	if err == nil {
		t.Fatal("red_count>0 MUST block the ship — got nil error (trust-kernel breach)")
	}
	se := wantShipErr(t, err, core.CodeEGPSRedCount, core.ShipClassPrecondition, "")
	// The refusal must name the RED predicate IDs so the operator can act.
	if !strings.Contains(se.Message, "pred-auth-leak") || !strings.Contains(se.Message, "pred-null-deref") {
		t.Errorf("EGPS refusal must name RED predicate IDs; got %q", se.Message)
	}
}

// TestCheckEGPSGate_MalformedJSON mirrors the bash "fall through silently"
// posture: a corrupt acs-verdict.json must NOT block the ship (the audit
// report's verdict is the authoritative signal in that degraded case).
func TestCheckEGPSGate_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "acs-verdict.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res := &RunResult{}
	if err := checkEGPSGate(path, res); err != nil {
		t.Fatalf("malformed JSON must be a soft no-op; got %v", err)
	}
}

// TestCheckEGPSGate_ReadError covers the non-ErrNotExist read failure
// branch by pointing the gate at a directory (os.ReadFile of a dir errors
// but is not os.ErrNotExist), which must surface as a wrapped error.
func TestCheckEGPSGate_ReadError(t *testing.T) {
	dir := t.TempDir() // a directory, not a file
	res := &RunResult{}
	err := checkEGPSGate(dir, res)
	if err == nil {
		t.Fatal("reading a directory as acs-verdict.json must error")
	}
	// Not an integrity error — it's a runtime read failure.
	var ie *IntegrityError
	if errors.As(err, &ie) {
		t.Errorf("read failure should be a plain error, not IntegrityError; got %v", err)
	}
}

// --- verifyTrivial -------------------------------------------------------

// TestVerifyTrivial_RejectsNonTrivialEstimate: --class trivial is only
// legal when cycle-state.json:cycle_size_estimate == "trivial".
func TestVerifyTrivial_RejectsNonTrivialEstimate(t *testing.T) {
	root := t.TempDir()
	writeCycleState(t, root, "small") // not "trivial"
	opts := &Options{ProjectRoot: root, Runner: (&scriptedRunner{}).runner()}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeTrivialNotTrivial, core.ShipClassConfig, "trivial")
}

// TestVerifyTrivial_RejectsPipelineCriticalPath: even a trivial-sized
// cycle cannot bypass audit if it touches a Tier-1 path (skills/, agents/,
// kernel scripts, profiles, plugin manifest). Driven via the untracked
// file list to avoid the scriptedRunner "git diff" key collision.
func TestVerifyTrivial_RejectsPipelineCriticalPath(t *testing.T) {
	root := t.TempDir()
	writeCycleState(t, root, "trivial")
	r := &scriptedRunner{}
	r.runner() // init map
	r.scripts["git ls-files"] = scriptResult(t, "skills/evolve-loop/SKILL.md\n", 0)
	opts := &Options{ProjectRoot: root, Runner: r.runner()}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeTrivialCriticalPaths, core.ShipClassConfig, "skills/evolve-loop/SKILL.md")
}

// TestVerifyTrivial_AcceptsCleanTrivialCycle: trivial estimate + no
// critical paths ⇒ pass with the skip-audit provenance recorded.
func TestVerifyTrivial_AcceptsCleanTrivialCycle(t *testing.T) {
	root := t.TempDir()
	writeCycleState(t, root, "trivial")
	r := &scriptedRunner{}
	r.runner()
	// All three file-list queries return only a non-critical file.
	r.scripts["git diff"] = scriptResult(t, "README.md\n", 0)
	r.scripts["git ls-files"] = scriptResult(t, "", 0)
	res := &RunResult{}
	opts := &Options{ProjectRoot: root, Runner: r.runner()}
	if err := verifyTrivial(context.Background(), opts, res); err != nil {
		t.Fatalf("clean trivial cycle must pass; got %v", err)
	}
	if res.Provenance != "trivial (skip-audit, kernel-verified)" {
		t.Errorf("provenance=%q want trivial skip-audit", res.Provenance)
	}
}

// --- verifyManualConfirm -------------------------------------------------

// TestVerifyManualConfirm_NothingStaged: when `git diff --cached --quiet`
// reports no staged changes (exit 0), ship exits cleanly via errEmptyDiff.
func TestVerifyManualConfirm_NothingStaged(t *testing.T) {
	r := &scriptedRunner{}
	r.runner()
	// git add -A → exit 0 (default); git diff --cached --quiet → exit 0 = nothing staged.
	r.scripts["git diff"] = scriptResult(t, "", 0)
	opts := &Options{ProjectRoot: t.TempDir(), Runner: r.runner(), Stderr: os.NewFile(0, os.DevNull)}
	err := verifyManualConfirm(context.Background(), opts, &RunResult{})
	if !errors.Is(err, errEmptyDiff) {
		t.Fatalf("no staged changes must return errEmptyDiff; got %v", err)
	}
}

// TestVerifyManualConfirm_AutoConfirm: EVOLVE_SHIP_AUTO_CONFIRM=1 bypasses
// the interactive prompt (CI mode) when there ARE staged changes.
func TestVerifyManualConfirm_AutoConfirm(t *testing.T) {
	r := &scriptedRunner{}
	r.runner()
	// git diff --cached --quiet → exit 1 = changes staged ⇒ proceed.
	r.scripts["git diff"] = scriptResult(t, "", 1)
	res := &RunResult{}
	opts := &Options{
		ProjectRoot: t.TempDir(),
		Runner:      r.runner(),
		Env:         map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
		Stderr:      os.NewFile(0, os.DevNull),
	}
	if err := verifyManualConfirm(context.Background(), opts, res); err != nil {
		t.Fatalf("auto-confirm must succeed; got %v", err)
	}
	if res.Provenance != "manual (auto-confirmed via env)" {
		t.Errorf("provenance=%q want auto-confirmed", res.Provenance)
	}
}

// --- helpers -------------------------------------------------------------

func writeACSVerdict(t *testing.T, doc map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "acs-verdict.json")
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal acs-verdict: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write acs-verdict: %v", err)
	}
	return path
}

func writeCycleState(t *testing.T, root, estimate string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	raw, _ := json.Marshal(map[string]any{"cycle_size_estimate": estimate})
	if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), raw, 0o644); err != nil {
		t.Fatalf("write cycle-state: %v", err)
	}
}

// scriptResult builds the anonymous-struct value scriptedRunner.scripts
// expects, keeping the call sites in this file readable.
func scriptResult(t *testing.T, stdout string, exit int) struct {
	stdout string
	stderr string
	exit   int
	err    error
} {
	t.Helper()
	return struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: stdout, exit: exit}
}
