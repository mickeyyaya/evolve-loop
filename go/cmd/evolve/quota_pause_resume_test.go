package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestQuotaPause_TheResumeDelayHasA60SecondFloorAndAOneHourCapForEverySourceThatAutoResumes(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 42, 13, 0, time.UTC)
	for _, source := range []string{"operator-override", "parsed", "bench", "usage", "default"} {
		for _, tc := range []struct {
			name string
			wake time.Time
			want time.Duration
		}{
			{"a wake time already past", now.Add(-time.Hour), 60 * time.Second},
			{"a wake time now", now, 60 * time.Second},
			{"a wake time in ten minutes", now.Add(10 * time.Minute), 11 * time.Minute},
			{"a wake time in five hours", now.Add(5 * time.Hour), time.Hour},
		} {
			qp := quotaPause{WakeAt: tc.wake.Format(time.RFC3339), Source: source, MaxAttempts: 3}

			got, ok := qp.resumeDelay(now)

			if !ok || got != tc.want {
				t.Errorf("%s, %s: delay=%v ok=%v, want %v", source, tc.name, got, ok, tc.want)
			}
		}
	}
}

func TestQuotaPause_ANonResumablePauseIsNeverScheduled(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 42, 13, 0, time.UTC)
	for _, tc := range []struct {
		name string
		qp   quotaPause
	}{
		{"an unknown reset", quotaPause{Source: "unknown", MaxAttempts: 0, OperatorAction: "resume by hand"}},
		{"the attempt cap reached", quotaPause{WakeAt: now.Format(time.RFC3339), Source: "bench", Attempts: 3, MaxAttempts: 3}},
		{"an unreadable wake time", quotaPause{WakeAt: "soon", Source: "operator-override", MaxAttempts: 3}},
	} {
		if d, ok := tc.qp.resumeDelay(now); ok {
			t.Errorf("%s: delay=%v, want no auto-resume", tc.name, d)
		}
	}
}

func TestQuotaPauseLine_SaysWhenToResumeOrThatTheOperatorMust(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 42, 13, 0, time.UTC)
	scheduled := quotaPauseLine(quotaPause{Cycle: 7, WakeAt: now.Add(10 * time.Minute).Format(time.RFC3339), Source: "bench", MaxAttempts: 3}, now)
	manual := quotaPauseLine(quotaPause{Cycle: 7, Source: "unknown", OperatorAction: "resume by hand"}, now)

	if !strings.HasPrefix(scheduled, "QUOTA-PAUSE: cycle=7 wake-at=") || !strings.Contains(scheduled, "resume-in=660s") {
		t.Errorf("scheduled line %q, want the wake-at and resume-in=660s", scheduled)
	}
	if !strings.Contains(manual, "auto-resume=off: resume by hand") || strings.Contains(manual, "resume-in=") {
		t.Errorf("manual line %q, want auto-resume=off with the operator action and no resume-in", manual)
	}
}

func TestObserveSequentialCycle_ASystemFailureHaltStopsTheBatchBeforeAnyQuotaPause(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: &console}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, &console)
	cycle := sequentialCycle{cycle: 1853, workspace: t.TempDir(), result: core.CycleResult{SystemFailure: &cyclestate.SystemFailureSignal{Category: "infra-systemic", Level: "system", Evidence: "pane_lost", Halt: true}}}

	d := b.observeSequentialCycle(cycle, &sequentialBatchState{})

	if d.flow != batchReturn || b.result.StopReason != "system_failure_halt" {
		t.Fatalf("decision=%+v stop=%q, want a returned system_failure_halt", d, b.result.StopReason)
	}
}
