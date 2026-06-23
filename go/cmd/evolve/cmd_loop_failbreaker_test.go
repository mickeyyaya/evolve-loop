package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/policy"
	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

// cmd_loop_failbreaker_test.go — EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS: the
// circuit-broken continue-on-verdict-FAIL. Soaks #3/#3b/#3c/#3d (2026-06-13)
// each ended on the FIRST cycle whose FinalVerdict was FAIL, even when the
// failure was a localized work-quality miss in an otherwise healthy batch —
// turning every miss into an operator relaunch and preventing any
// 3-consecutive-PASS streak from forming. The flag lets a batch absorb up to
// max-1 consecutive FAILs (a streak of PASS/SHIPPED resets the count); the
// default of 1 reproduces the pre-flag stop-on-first-FAIL contract exactly.

func TestConsecutiveFailBreaker(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		failed     bool
		streak     int
		max        int
		wantStreak int
		wantStop   bool
	}{
		// Default max=1: any FAIL stops immediately (byte-identical to the
		// pre-flag unconditional break).
		{"default max=1 stops on first fail", true, 0, 1, 1, true},
		// max=3: first two fails continue, third stops.
		{"max=3 first fail continues", true, 0, 3, 1, false},
		{"max=3 second fail continues", true, 1, 3, 2, false},
		{"max=3 third fail stops", true, 2, 3, 3, true},
		// A non-FAIL cycle resets the streak — two fails then a pass then a
		// fail must NOT stop at max=3 (the pass broke the run).
		{"pass resets streak", false, 2, 3, 0, false},
		{"fail after reset is streak 1", true, 0, 3, 1, false},
		// Streak already at/over max keeps stopping (defensive).
		{"streak past max stays stopped", true, 5, 3, 6, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, stop := consecutiveFailBreaker(tc.failed, tc.streak, tc.max)
			if s != tc.wantStreak || stop != tc.wantStop {
				t.Fatalf("consecutiveFailBreaker(%v,%d,%d) = (streak=%d stop=%v), want (streak=%d stop=%v)",
					tc.failed, tc.streak, tc.max, s, stop, tc.wantStreak, tc.wantStop)
			}
		})
	}
}

func TestResolveMaxConsecutiveFails(t *testing.T) {
	tests := []struct {
		name     string
		workflow *policy.WorkflowPolicy
		want     int
	}{
		{"absent defaults to 1", nil, 1},
		{"valid value", &policy.WorkflowPolicy{MaxConsecutiveFails: 3}, 3},
		{"zero clamps to default", &policy.WorkflowPolicy{}, 1},
		{"negative clamps to default", &policy.WorkflowPolicy{MaxConsecutiveFails: -2}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := (policy.Policy{Workflow: tc.workflow}).WorkflowConfig().MaxConsecutiveFails; got != tc.want {
				t.Errorf("WorkflowConfig().MaxConsecutiveFails = %d, want %d", got, tc.want)
			}
		})
	}
}

func countFailedApproaches(t *testing.T, statePath string) int {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var st struct {
		FailedApproaches []map[string]any `json:"failedApproaches"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal state.json: %v", err)
	}
	return len(st.FailedApproaches)
}

// TestRecordAbsorbedFail is the regression pin for the continue-on-fail BLOCK
// finding: an absorbed verdict-FAIL must be written to
// state.json:failedApproaches. The orchestrator does NOT record the
// clean-completion FAIL path (only err!=nil cycle-level failures), and the
// stop-path recordLoopFatal is skipped when continuing — so without this the
// next cycle's scout would see no record of the failure.
func TestRecordAbsorbedFail(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".evolve", "runs", "cycle-7"), 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(evolveDir, "state.json")
	if err := os.WriteFile(statePath, []byte(`{"failedApproaches":[],"lastCycleNumber":0}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loopConfig{ProjectRoot: projectRoot, EvolveDir: evolveDir}
	before := countFailedApproaches(t, statePath)

	var stderr bytes.Buffer
	recordAbsorbedFail(cfg, 7, &stderr)

	if after := countFailedApproaches(t, statePath); after != before+1 {
		t.Fatalf("absorbed FAIL must append exactly one failedApproaches entry: before=%d after=%d, stderr=%q",
			before, after, stderr.String())
	}
}

// TestRecordAbsorbedFail_MissingStateIsSoftWarn pins that a missing state.json
// does not panic or hard-fail — the loop must keep running past an absorbed
// FAIL even when pre-flight never created state.json.
func TestRecordAbsorbedFail_MissingStateIsSoftWarn(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := loopConfig{ProjectRoot: projectRoot, EvolveDir: filepath.Join(projectRoot, ".evolve")}
	var stderr bytes.Buffer
	recordAbsorbedFail(cfg, 1, &stderr) // no state.json on disk — must not panic
}

// failingOrch is a loopCycleRunner that returns a fixed FinalVerdict with an
// incrementing cycle number — the seam that lets the call-site wiring of the
// consecutive-fail breaker be exercised end-to-end (the real *core.Orchestrator
// cannot be scripted to emit FinalVerdict=FAIL without a faithful phase
// machine). Injected via loopOrchOverride.
type failingOrch struct {
	verdict string
	n       int
}

func (f *failingOrch) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	f.n++
	return core.CycleResult{Cycle: f.n, FinalVerdict: f.verdict}, nil
}
func (f *failingOrch) RunCycleFromPhase(ctx context.Context, req core.CycleRequest, _ *core.ResumePoint) (core.CycleResult, error) {
	return f.RunCycle(ctx, req)
}

func runFailLoop(t *testing.T, maxConsecutive string) (int, string) {
	t.Helper()

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	maxFails, err := strconv.Atoi(maxConsecutive)
	if err != nil {
		t.Fatalf("parse max consecutive fails: %v", err)
	}
	policyJSON := fmt.Sprintf(`{"dispatch":{"policy":"off"},"workflow":{"max_consecutive_fails":%d}}`, maxFails)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(`{"failedApproaches":[],"lastCycleNumber":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()
	prev := loopOrchOverride
	loopOrchOverride = &failingOrch{verdict: core.VerdictFAIL}
	defer func() { loopOrchOverride = prev }()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "fail goal",
		"--cycles", "5",
	}, nil, &stdout, &stderr)
	return rc, stderr.String()
}

// TestRunLoop_ContinueOnFail_CallSite is the end-to-end regression pin for the
// continue-on-fail WIRING (not just the pure helpers): with the default max=1
// the batch stops on the first FAIL; with max=3 it absorbs FAILs and keeps
// running, emitting the "continuing" line. Guards against a future loop edit
// silently dropping the breaker/record call.
func TestRunLoop_ContinueOnFail_CallSite(t *testing.T) {
	t.Run("default max=1 stops on first FAIL", func(t *testing.T) {
		_, stderr := runFailLoop(t, "1")
		if strings.Contains(stderr, "continuing (consecutive") {
			t.Errorf("default must NOT continue past a FAIL; stderr=%q", stderr)
		}
	})
	t.Run("max=3 absorbs and continues", func(t *testing.T) {
		_, stderr := runFailLoop(t, "3")
		if !strings.Contains(stderr, "continuing (consecutive 1 of max 3") {
			t.Errorf("max=3 must absorb the first FAIL and continue; stderr=%q", stderr)
		}
		if !strings.Contains(stderr, "consecutive 2 of max 3") {
			t.Errorf("max=3 must absorb the second FAIL too; stderr=%q", stderr)
		}
	})
}
