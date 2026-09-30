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

func TestCheckEGPSGate_MissingFile(t *testing.T) {
	res := &RunResult{}
	if _, err := checkEGPSGate(filepath.Join(t.TempDir(), "acs-verdict.json"), res); err == nil {
		t.Fatal("missing predicate evidence must refuse ship")
	}
	if len(res.Logs) != 0 {
		t.Errorf("missing file must not append logs; got %v", res.Logs)
	}
}

func TestCheckEGPSGate_RedCountZero(t *testing.T) {
	path := writeACSVerdict(t, predicateVerdictFixture(1, 12, 0, 0))
	res := &RunResult{}
	if _, err := checkEGPSGate(path, res); err != nil {
		t.Fatalf("red_count==0 must pass; got %v", err)
	}
	if len(res.Logs) == 0 || !strings.Contains(res.Logs[len(res.Logs)-1], "EGPS predicate suite verdict=PASS") {
		t.Errorf("expected EGPS-OK log line; got %v", res.Logs)
	}
}

func TestCheckEGPSGate_RedCountNonZero(t *testing.T) {
	v := predicateVerdictFixture(1, 5, 2, 0)
	v.RedIDs = []string{"pred-auth-leak", "pred-null-deref"}
	v.Results[5].ACID, v.Results[6].ACID = v.RedIDs[0], v.RedIDs[1]
	path := writeACSVerdict(t, v)
	res := &RunResult{}
	_, err := checkEGPSGate(path, res)
	if err == nil {
		t.Fatal("red_count>0 MUST block the ship — got nil error (trust-kernel breach)")
	}
	se := wantShipErr(t, err, core.CodeEGPSRedCount, core.ShipClassPrecondition, "")
	if !strings.Contains(se.Message, "pred-auth-leak") || !strings.Contains(se.Message, "pred-null-deref") {
		t.Errorf("EGPS refusal must name RED predicate IDs; got %q", se.Message)
	}
}

func TestCheckEGPSGate_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "acs-verdict.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res := &RunResult{}
	if _, err := checkEGPSGate(path, res); err == nil {
		t.Fatal("malformed predicate evidence must refuse ship")
	}
}

// TestCheckEGPSGate_ReadError points the gate at a directory: os.ReadFile of a
// dir errors but is not os.ErrNotExist, exercising the non-ErrNotExist branch.
func TestCheckEGPSGate_ReadError(t *testing.T) {
	dir := t.TempDir()
	res := &RunResult{}
	_, err := checkEGPSGate(dir, res)
	if err == nil {
		t.Fatal("reading a directory as acs-verdict.json must error")
	}
	var ie *IntegrityError
	if errors.As(err, &ie) {
		t.Errorf("read failure should be a plain error, not IntegrityError; got %v", err)
	}
}

// --- verifyTrivial -------------------------------------------------------

func TestVerifyTrivial_RejectsNonTrivialEstimate(t *testing.T) {
	root := t.TempDir()
	writeCycleState(t, root, "small")
	opts := &Options{ProjectRoot: root, Runner: (&scriptedRunner{}).runner()}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeTrivialNotTrivial, core.ShipClassConfig, "trivial")
}

// TestVerifyTrivial_RejectsPipelineCriticalPath drives the critical path via
// the untracked file list, to avoid the scriptedRunner "git diff" key collision.
func TestVerifyTrivial_RejectsPipelineCriticalPath(t *testing.T) {
	root := t.TempDir()
	writeCycleState(t, root, "trivial")
	r := &scriptedRunner{}
	r.runner() // init map
	r.scripts["git ls-files"] = scriptResult(t, "skills/loop/SKILL.md\n", 0)
	opts := &Options{ProjectRoot: root, Runner: r.runner()}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeTrivialCriticalPaths, core.ShipClassConfig, "skills/loop/SKILL.md")
}

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

func writeACSVerdict(t *testing.T, doc any) string {
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

// scriptResult builds the anonymous-struct value scriptedRunner.scripts expects.
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
