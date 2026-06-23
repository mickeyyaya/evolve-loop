package bridge

// askbroker_rung_test.go — ADR-0045 I3 (§8): the pre-85 AskBroker rung inside
// the auto-respond escalate branch. White-box: drives a real autoResponder
// with a manifest rule whose policy is `escalate` (so a matching pane yields
// rc 85), and a KernelAnswerer whose facts cover the question.

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/interaction"
)

// escalatePrompt is a manifest rule that escalates (no response keys) — the
// pre-fix path returns rc 85 straight to ExitUnknownPrompt.
var escalatePrompt = []ManifestPrompt{{
	Name:   "unknown_path_question",
	Regex:  "Which absolute path should I write the deliverable to\\?",
	Policy: "escalate",
}}

func brokerResponder(t *testing.T, panes []string, stage string, facts interaction.KernelFacts) (*autoResponder, *fakeTmux, *interaction.Recorder) {
	t.Helper()
	ws := t.TempDir()
	tmux := &fakeTmux{paneSeq: panes}
	deps := Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
	rec := interaction.NewRecorder(ws)
	ar := newAutoResponder("claude-tmux", ws, deps, false, 0)
	ar.prompts = escalatePrompt
	ar.rec, ar.phase, ar.cycle = rec, "build", 7
	ar.broker = interaction.NewKernelAnswerer(facts)
	ar.brokerStage = stage
	return ar, tmux, rec
}

const blockedQ = "Which absolute path should I write the deliverable to?"

// TestPre85Rung_KernelHitInjectsOnce_ClearedContinues — enforce: the kernel
// knows the artifact path, so the rung injects it (rc 1, not 85) exactly once;
// a second escalation (brokerTried) falls through to rc 85.
func TestPre85Rung_KernelHitInjectsOnce_ClearedContinues(t *testing.T) {
	t.Parallel()
	facts := interaction.KernelFacts{ArtifactPath: "/ws/cycle-7/build-report.md"}
	// Tick 1 pane: the blocking question (escalates). Tick 2 pane: same
	// question still present (agent hasn't moved) → second escalation.
	ar, tmux, rec := brokerResponder(t, []string{blockedQ, blockedQ}, "enforce", facts)
	ctx := context.Background()

	if _, rc := ar.tick(ctx, "s"); rc != 1 {
		t.Fatalf("tick 1: kernel hit must inject and return rc 1 (responded), not escalate; got rc=%d", rc)
	}
	if !tmux.sentContains("/ws/cycle-7/build-report.md") {
		t.Errorf("the kernel answer (artifact path) must have been injected; sent=%v", tmux.sentKeys)
	}
	if _, rc := ar.tick(ctx, "s"); rc != 85 {
		t.Fatalf("tick 2: the broker is once-per-launch — a still-stuck pane must now escalate (rc 85); got rc=%d", rc)
	}
	ar.flushPending()
	var kernel int
	for _, o := range rec.Outcomes() {
		if o.Kind == interaction.KindKernelAnswer {
			kernel++
			if o.Trigger != "unknown_prompt" {
				t.Errorf("kernel outcome trigger = %q", o.Trigger)
			}
		}
	}
	if kernel != 1 {
		t.Errorf("exactly one kernel_answer outcome must be recorded; got %d", kernel)
	}
}

// TestPre85Rung_MissFallsThroughToFallbackChainUnchanged — the kernel does
// NOT know the answer ⇒ the rung returns false and the escalation fires
// (rc 85), the 85 → fallback chain untouched (B1, the unconditional floor).
func TestPre85Rung_MissFallsThroughToFallbackChainUnchanged(t *testing.T) {
	t.Parallel()
	// Empty facts → every question misses.
	ar, tmux, _ := brokerResponder(t, []string{blockedQ}, "enforce", interaction.KernelFacts{})
	if _, rc := ar.tick(context.Background(), "s"); rc != 85 {
		t.Fatalf("a kernel miss must fall through to rc 85 unchanged; got rc=%d", rc)
	}
	if tmux.sentContains("/ws") {
		t.Errorf("nothing may be injected on a miss; sent=%v", tmux.sentKeys)
	}
}

// TestPre85Rung_ShadowDoesNotInject — shadow records a would-act soak signal
// but injects nothing and still escalates (byte-identical behavior).
func TestPre85Rung_ShadowDoesNotInject(t *testing.T) {
	t.Parallel()
	facts := interaction.KernelFacts{ArtifactPath: "/ws/cycle-7/build-report.md"}
	ar, tmux, rec := brokerResponder(t, []string{blockedQ}, "shadow", facts)
	if _, rc := ar.tick(context.Background(), "s"); rc != 85 {
		t.Fatalf("shadow must still escalate (rc 85); got rc=%d", rc)
	}
	if len(tmux.sentKeys) != 0 {
		t.Errorf("shadow must inject nothing; sent=%v", tmux.sentKeys)
	}
	sawWouldAct := false
	for _, o := range rec.Outcomes() {
		if o.Kind == interaction.KindKernelAnswer && o.Result == interaction.ResultWouldAct {
			sawWouldAct = true
		}
	}
	if !sawWouldAct {
		t.Error("shadow must record a kernel_answer would_act soak signal")
	}
}

// TestPre85Rung_ShadowSoakRecordsEachTick — the once-budget bounds INJECTION
// (enforce), not soak recording: a still-stuck shadow pane records a would_act
// on each reaching tick (so the I1 soak signal isn't capped at one), and never
// injects.
func TestPre85Rung_ShadowSoakRecordsEachTick(t *testing.T) {
	t.Parallel()
	facts := interaction.KernelFacts{ArtifactPath: "/ws/cycle-7/build-report.md"}
	ar, tmux, rec := brokerResponder(t, []string{blockedQ, blockedQ}, "shadow", facts)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, rc := ar.tick(ctx, "s"); rc != 85 {
			t.Fatalf("tick %d shadow must escalate (rc 85); got %d", i, rc)
		}
	}
	if len(tmux.sentKeys) != 0 {
		t.Errorf("shadow must never inject; sent=%v", tmux.sentKeys)
	}
	var wouldAct int
	for _, o := range rec.Outcomes() {
		if o.Kind == interaction.KindKernelAnswer && o.Result == interaction.ResultWouldAct {
			wouldAct++
		}
	}
	if wouldAct != 2 {
		t.Errorf("shadow soak must record each reaching tick; got %d would_act, want 2", wouldAct)
	}
}

// TestPre85Rung_OffStageInert — off: no broker action at all, escalates as
// today with no would-act noise.
func TestPre85Rung_OffStageInert(t *testing.T) {
	t.Parallel()
	facts := interaction.KernelFacts{ArtifactPath: "/ws/cycle-7/build-report.md"}
	ar, tmux, rec := brokerResponder(t, []string{blockedQ}, "off", facts)
	if _, rc := ar.tick(context.Background(), "s"); rc != 85 {
		t.Fatalf("off must escalate (rc 85); got rc=%d", rc)
	}
	if len(tmux.sentKeys) != 0 || len(rec.Outcomes()) != 0 {
		t.Errorf("off must be fully inert; sent=%v outcomes=%v", tmux.sentKeys, rec.Outcomes())
	}
}
