package phaseobserver

// stallpolicy_test.go — ADR-0044 C4 (Slice 4) RED tests: the observer's
// StallPolicy Strategy seam.
//
// cycle-262 D5: the observer DETECTS stalls (stuck_no_output /
// stuck_no_progress INCIDENT events) but its only action is an inline
// "Enforce → SIGTERM" branch — detection and action are welded together, so
// recovery policy can't evolve without editing the detector (SRP violation).
// C4 extracts the action decision into recovery.StallPolicy injected via
// Config: nil policy ⇒ byte-identical legacy behavior (Enforce branch,
// unenriched envelope); a policy maps each typed StallEvent →
// extend | kill_retry | escalate, the decision is recorded INSIDE the
// INCIDENT envelope (action + action_reason — every recovery decision is
// justified, ADR-0044), and only kill_retry touches the process group.
//
// The policy is wired by the C3 composition slice; this slice ships the seam
// default-nil (behavior-neutral), pinned by the legacy tests above plus
// TestRun_StallPolicyNil_EnvelopeUnenriched below.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// scriptedStallPolicy returns a fixed action for every stall incident and
// records the events it was consulted with.
type scriptedStallPolicy struct {
	mu     sync.Mutex
	action recovery.StallAction
	reason string
	seen   []recovery.StallEvent
}

func (p *scriptedStallPolicy) Decide(ev recovery.StallEvent) (recovery.StallAction, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, ev)
	return p.action, p.reason
}

// stallHarness runs the observer against a stalled (empty) stdout with a
// frozen-then-jumped clock so the stall rule fires deterministically. It
// returns the kill-call count and the raw events file.
func stallHarness(t *testing.T, enforce bool, policy *scriptedStallPolicy) (int, string) {
	t.Helper()
	ws := tempWorkspace(t)
	killCalls := 0
	mu := &sync.Mutex{}
	startTime := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	callIdx := 0
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		callIdx++
		if callIdx <= 2 {
			return startTime
		}
		return startTime.Add(1000 * time.Second)
	}
	cfg := Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 262,
		Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, EOFGraceS: 9999,
		Enforce: enforce,
		Now:     nowFn,
		KillPgrp: func(int, syscall.Signal) error {
			mu.Lock()
			killCalls++
			mu.Unlock()
			return nil
		},
		StopAfterMS: 800,
	}
	if policy != nil {
		cfg.StallPolicy = policy
	}
	rc := Run(cfg, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	events, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatalf("events file missing: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	return killCalls, string(events)
}

// TestRun_StallPolicyExtend_NoKill: a policy verdict of extend outranks the
// legacy Enforce kill — the agent keeps running, and the decision is
// justified inside the INCIDENT envelope.
func TestRun_StallPolicyExtend_NoKill(t *testing.T) {
	t.Parallel()
	pol := &scriptedStallPolicy{action: recovery.StallExtend, reason: "deep-thinking phase; extend"}
	kills, events := stallHarness(t, true /* Enforce — policy must override */, pol)
	if kills != 0 {
		t.Errorf("policy=extend must suppress the kill even with Enforce=true; got %d kill(s)", kills)
	}
	if !strings.Contains(events, `"action":"extend"`) {
		t.Errorf("INCIDENT envelope must record the policy action; events:\n%s", events)
	}
	if !strings.Contains(events, "deep-thinking phase; extend") {
		t.Errorf("INCIDENT envelope must record the justification; events:\n%s", events)
	}
	pol.mu.Lock()
	defer pol.mu.Unlock()
	if len(pol.seen) == 0 {
		t.Fatal("policy was never consulted")
	}
	if pol.seen[0].Kind != "stuck_no_output" || pol.seen[0].Phase != "build" {
		t.Errorf("policy must receive the typed stall event; got %+v", pol.seen[0])
	}
}

// TestRun_StallPolicyKillRetry_SendsSIGTERM: kill_retry kills the process
// group on the policy's authority — Enforce=false must not veto it (the
// policy IS the action decision once injected).
func TestRun_StallPolicyKillRetry_SendsSIGTERM(t *testing.T) {
	t.Parallel()
	pol := &scriptedStallPolicy{action: recovery.StallKillRetry, reason: "dead pane; fresh dispatch"}
	kills, events := stallHarness(t, false /* no Enforce — policy drives */, pol)
	if kills == 0 {
		t.Error("policy=kill_retry must SIGTERM the pgid even without Enforce")
	}
	if !strings.Contains(events, `"action":"kill_retry"`) {
		t.Errorf("INCIDENT envelope must record the kill decision; events:\n%s", events)
	}
}

// TestRun_StallPolicyEscalate_NoKill: escalate surfaces without acting.
func TestRun_StallPolicyEscalate_NoKill(t *testing.T) {
	t.Parallel()
	pol := &scriptedStallPolicy{action: recovery.StallEscalate, reason: "integrity-adjacent; operator decides"}
	kills, events := stallHarness(t, true, pol)
	if kills != 0 {
		t.Errorf("policy=escalate must not kill; got %d", kills)
	}
	if !strings.Contains(events, `"action":"escalate"`) {
		t.Errorf("INCIDENT envelope must record escalate; events:\n%s", events)
	}
}

// TestRun_StallPolicyNil_EnvelopeUnenriched pins byte-identical legacy
// behavior for the default-nil seam: the Enforce branch still kills (covered
// by TestRun_StallDetectionFires) AND the INCIDENT envelope carries NO
// action/action_reason keys — the enrichment is policy-only.
func TestRun_StallPolicyNil_EnvelopeUnenriched(t *testing.T) {
	t.Parallel()
	kills, events := stallHarness(t, true, nil)
	if kills == 0 {
		t.Error("nil policy + Enforce must keep the legacy kill")
	}
	if strings.Contains(events, `"action"`) || strings.Contains(events, "action_reason") {
		t.Errorf("nil policy must leave the INCIDENT envelope byte-identical (no action keys); events:\n%s", events)
	}
}
