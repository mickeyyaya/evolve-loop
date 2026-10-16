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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

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
		{"default max=1 stops on first fail", true, 0, 1, 1, true},
		{"max=3 first fail continues", true, 0, 3, 1, false},
		{"max=3 second fail continues", true, 1, 3, 2, false},
		{"max=3 third fail stops", true, 2, 3, 3, true},
		{"pass resets streak", false, 2, 3, 0, false},
		{"fail after reset is streak 1", true, 0, 3, 1, false},
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
	t.Parallel()
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

func TestRecordAbsorbedFail(t *testing.T) {
	t.Parallel()
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

func TestRecordAbsorbedFail_MissingStateIsSoftWarn(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	cfg := loopConfig{ProjectRoot: projectRoot, EvolveDir: filepath.Join(projectRoot, ".evolve")}
	var stderr bytes.Buffer
	recordAbsorbedFail(cfg, 1, &stderr)
}

type failingOrch struct {
	noSignals
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
