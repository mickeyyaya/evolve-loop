package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type recordingHostEffects struct{ ins []core.ReviewInput }

func (r *recordingHostEffects) Perform(_ context.Context, in core.ReviewInput) error {
	r.ins = append(r.ins, in)
	return nil
}

func TestRun_HostEffectsPrecedeTheFirstVerification(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	const report = "# Audit Report\n\n## Verdict\n**PASS**\n"
	effects := &recordingHostEffects{}
	verifies := 0
	verify := func(phase string, _ phasecontract.Roots) (deliverable.Result, error) {
		verifies++
		if len(effects.ins) == 0 {
			return deliverable.Result{Phase: phase, Violations: []deliverable.Violation{{Code: deliverable.CodeMissingEffect, Message: "not performed"}}}, nil
		}
		return deliverable.Result{OK: true, Phase: phase, Content: report}, nil
	}
	r := New(Options{
		Hooks: hooks, Bridge: &divergentBridge{fileContent: report, stdoutContent: report},
		Prompts: fakePromptsFS("evolve-auditor", "x"), VerifyFn: verify,
		HostEffects: func() core.HostEffects { return effects },
		SleepFn:     func(time.Duration) {},
	})
	if !r.HostEffectsWired() {
		t.Fatal("the injected host effects must be reported wired")
	}
	ws := t.TempDir()
	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: ws, Cycle: 7})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS || verifies != 1 {
		t.Fatalf("host effects must run before the first verification: verdict=%s verifications=%d", resp.Verdict, verifies)
	}
	if len(effects.ins) != 1 || effects.ins[0].Phase != "audit" || effects.ins[0].Cycle != 7 || effects.ins[0].Workspace != ws || effects.ins[0].ProjectRoot != root {
		t.Fatalf("host effects receive the dispatch's projection once: %+v", effects.ins)
	}
}

type failingHostEffects struct{}

func (failingHostEffects) Perform(context.Context, core.ReviewInput) error {
	return errors.New("inbox move failed")
}

func TestRun_AFailedHostEffectIsACodedWarning(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	const report = "# Audit Report\n\n## Verdict\n**PASS**\n"
	center := signalcenter.New()
	var got []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) {
		if e.Code == CodeHostEffectFailed {
			got = append(got, e)
		}
	})
	r := New(Options{
		Hooks: hooks, Bridge: &divergentBridge{fileContent: report, stdoutContent: report},
		Prompts: fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: func(phase string, _ phasecontract.Roots) (deliverable.Result, error) {
			return deliverable.Result{OK: true, Phase: phase, Content: report}, nil
		},
		HostEffects: func() core.HostEffects { return failingHostEffects{} },
		Signals:     func() *signalcenter.Center { return center },
		SleepFn:     func(time.Duration) {},
	})
	if _, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir(), Cycle: 7}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityWarn || got[0].Cycle != 7 || got[0].Phase != "audit" || !strings.Contains(got[0].Reason, "inbox move failed") {
		t.Fatalf("signals = %+v, want one WARN %s for audit in cycle 7 carrying the cause", got, CodeHostEffectFailed)
	}
}
