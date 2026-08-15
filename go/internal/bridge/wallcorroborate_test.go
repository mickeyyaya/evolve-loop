package bridge

// wallcorroborate_test.go — contracts for the wall-corroboration seam. The
// incident narrative and design rationale live ONCE, in wallcorroborate.go's
// header (2026-08-15 false-wall class; Strategy via DI, nil = legacy).

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// corroboratorProbeRunner scripts the probe subprocess and records argv/stdin.
type corroboratorProbeRunner struct {
	calls  [][]string
	stdins []string
	rc     int
}

func (r *corroboratorProbeRunner) run(_ context.Context, name, _ string, args, _ []string,
	stdin io.Reader, _, _ io.Writer) (int, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	var in string
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		in = string(b)
	}
	r.stdins = append(r.stdins, in)
	return r.rc, nil
}

// TestWallCorroborator_NilMeansLegacyVerdict names the strategy TYPE
// (apicover) and pins the seam's zero value: nil = the pane match IS the
// verdict, byte-identical to pre-fix behavior for every un-wired caller.
func TestWallCorroborator_NilMeansLegacyVerdict(t *testing.T) {
	t.Parallel()
	var c WallCorroborator
	if !wallCorroborated(context.Background(), c, "claude-tmux") {
		t.Fatal("nil corroborator must preserve the legacy verdict (walled=true)")
	}
}

func TestDefaultWallCorroborator_ProbeSuccessMeansNotWalled(t *testing.T) {
	t.Parallel()
	r := &corroboratorProbeRunner{rc: 0}
	c := DefaultWallCorroborator(r.run, io.Discard)
	if walled := c(context.Background(), "claude-tmux"); walled {
		t.Fatal("a succeeding live probe is proof the provider serves requests — not walled")
	}
	if len(r.calls) != 1 || r.calls[0][0] != "claude" {
		t.Fatalf("claude probe must invoke the claude binary: %v", r.calls)
	}
	joined := strings.Join(r.calls[0], " ")
	if !strings.Contains(joined, "-p") || !strings.Contains(joined, "haiku") {
		t.Errorf("claude probe must be a one-shot headless -p call on the cheapest tier: %v", r.calls[0])
	}
}

func TestDefaultWallCorroborator_ProbeFailureMeansWalled(t *testing.T) {
	t.Parallel()
	r := &corroboratorProbeRunner{rc: 1}
	c := DefaultWallCorroborator(r.run, io.Discard)
	if walled := c(context.Background(), "claude-tmux"); !walled {
		t.Fatal("a failing live probe corroborates the wall — the fast-fail must proceed")
	}
}

func TestDefaultWallCorroborator_CodexRecipeUsesExecWithStdin(t *testing.T) {
	t.Parallel()
	r := &corroboratorProbeRunner{rc: 0}
	c := DefaultWallCorroborator(r.run, io.Discard)
	if walled := c(context.Background(), "codex-tmux"); walled {
		t.Fatal("succeeding codex probe ⇒ not walled")
	}
	if len(r.calls) != 1 || r.calls[0][0] != "codex" || !strings.Contains(strings.Join(r.calls[0], " "), "exec") {
		t.Fatalf("codex probe must be the headless `codex exec -` form: %v", r.calls)
	}
	if len(r.stdins) != 1 || strings.TrimSpace(r.stdins[0]) == "" {
		t.Fatalf("codex exec - reads the prompt from stdin; the probe must supply one: %q", r.stdins)
	}
}

// Families without a declared probe recipe stay CONSERVATIVE: corroborated
// walled (legacy behavior), and no subprocess is ever invented for them.
func TestDefaultWallCorroborator_UnknownFamilyStaysConservative(t *testing.T) {
	t.Parallel()
	r := &corroboratorProbeRunner{rc: 0}
	c := DefaultWallCorroborator(r.run, io.Discard)
	if walled := c(context.Background(), "ollama-tmux"); !walled {
		t.Fatal("no probe recipe ⇒ cannot corroborate ⇒ conservative walled=true (legacy)")
	}
	if len(r.calls) != 0 {
		t.Fatalf("no recipe must mean NO invented subprocess: %v", r.calls)
	}
}

// The probe is deadline-bounded: a hung probe must return walled=true (the
// provider not answering IS the wall signature) rather than hang the poll.
func TestDefaultWallCorroborator_HungProbeIsWalled(t *testing.T) {
	// NOT parallel — this test WRITES the package-level wallProbeTimeout var;
	// Go runs serial tests to completion before any t.Parallel test resumes,
	// which is the only ordering that makes the write race-free (the -race
	// CI run on PR #466 caught the parallel variant racing every reader).
	hang := func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		<-ctx.Done()
		return 1, ctx.Err()
	}
	old := wallProbeTimeout
	wallProbeTimeout = 50 * time.Millisecond
	defer func() { wallProbeTimeout = old }()
	c := DefaultWallCorroborator(hang, io.Discard)
	done := make(chan bool, 1)
	go func() { done <- c(context.Background(), "claude-tmux") }()
	select {
	case walled := <-done:
		if !walled {
			t.Fatal("a probe that cannot answer within the deadline corroborates the wall")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("corroborator hung past its own deadline — it must bound the probe")
	}
}

// --- tick-level wiring: the two decision sites ---

// A pane whose wall text is FILE CONTENT (not prompt echo — the 2026-08-15
// class: fixtures under edit persist frame after frame) must NOT escalate
// when the corroborator proves the provider healthy; the suppression is loud
// and the scan disables for the rest of the phase (exactly ONE probe, not one
// per tick).
func TestTick_ContentWallSuppressedWhenCorroboratorSaysHealthy(t *testing.T) {
	pane := "editing usageclassify_test.go\nfixture: \"You have reached your usage limit\"\nrunning tests...\n"
	var log strings.Builder
	probe := &corroboratorProbeRunner{rc: 0}
	deps := Deps{Tmux: &fakeTmux{paneSeq: []string{pane}}, Sleep: func(time.Duration) {},
		LookupEnv: mapLookup(nil), Stderr: &log,
		CorroborateWall: DefaultWallCorroborator(probe.run, &log)}.withDefaults()
	ar := newAutoResponder("claude-tmux", t.TempDir(), deps, false, 0)
	ar.prompts = nil
	ar.exhaustedRegex = c672ExhaustedPattern
	ar.injectedPrompt = "Instructions: fix the exhaustion regex."

	var rc int
	for i := 0; i < exhaustionPersistObservations+3; i++ {
		_, rc = ar.tick(context.Background(), "s")
		if rc == 85 {
			t.Fatalf("tick %d escalated rc 85 despite a healthy corroboration — the false all-families wall class", i)
		}
	}
	if len(probe.calls) != 1 {
		t.Fatalf("corroboration must run exactly ONCE per phase (then the scan disables), got %d probes", len(probe.calls))
	}
	if !strings.Contains(log.String(), "content-induced") {
		t.Errorf("suppression must be LOUD and name the class; log=%q", log.String())
	}
}

// A genuine wall — corroborator confirms — must still escalate exactly as
// before: the seam must never blanket-disable the fast-fail.
func TestTick_CorroboratedWallStillEscalates(t *testing.T) {
	pane := "You have reached your usage limit. Resets in 4h.\n"
	probe := &corroboratorProbeRunner{rc: 1}
	deps := Deps{Tmux: &fakeTmux{paneSeq: []string{pane}}, Sleep: func(time.Duration) {},
		LookupEnv:       mapLookup(nil),
		CorroborateWall: DefaultWallCorroborator(probe.run, io.Discard)}.withDefaults()
	ar := newAutoResponder("claude-tmux", t.TempDir(), deps, false, 0)
	ar.prompts = nil
	ar.exhaustedRegex = c672ExhaustedPattern
	ar.injectedPrompt = "Instructions: do the task."

	var rc int
	for i := 0; i < exhaustionPersistObservations; i++ {
		_, rc = ar.tick(context.Background(), "s")
	}
	if rc != 85 {
		t.Fatalf("corroborated real wall must escalate rc 85, got %d", rc)
	}
}

// --- taxonomy: burst/display vocabulary out of the exhaustion regexes ---

// The manifests' exhaustion regexes must match WINDOW-EXHAUSTION wording only.
// A 429 burst ("Too many requests") recovers in minutes — benching a family or
// checkpointing a batch on it is the wrong recovery (operator directive), and
// the usage DISPLAY's own labels ("Rate limits: … % left") are subject-matter
// vocabulary, not state (the false-bench class fixed at the probe layer).
func TestManifestExhaustedRegexes_WindowWallOnlyNeverBurstOrDisplay(t *testing.T) {
	t.Parallel()
	mustNotMatch := []string{
		"Too many requests — please slow down",
		"Rate limits: 5h limit: 88% left · weekly: 61% left",
		"429 too many requests, retrying in 20s",
	}
	mustMatch := map[string]string{
		"claude-tmux": "You have reached your usage limit",
		"codex-tmux":  "usage limit reached",
		"agy-tmux":    "quota exceeded",
	}
	for cli, wall := range mustMatch {
		m, err := LoadManifest(cli)
		if err != nil {
			t.Fatalf("load %s: %v", cli, err)
		}
		spec, ok := m.Control("usage")
		if !ok || spec.ExhaustedRegex == "" {
			t.Fatalf("%s must declare a usage exhausted_regex", cli)
		}
		if !matchExhausted(spec.ExhaustedRegex, wall) {
			t.Errorf("%s regex lost its TRUE wall match %q", cli, wall)
		}
		for _, benign := range mustNotMatch {
			if matchExhausted(spec.ExhaustedRegex, benign) {
				t.Errorf("%s regex matches burst/display vocabulary %q — the 429-as-wall / display-as-wall class", cli, benign)
			}
		}
	}
}

// A REAL wall must corroborate exactly ONCE per responder even when the
// caller discards rc (boot loop, recipe adapter, capture ticks): the gate
// LATCHES, so without a probed-latch every subsequent tick would re-fire a
// 60s quota-consuming probe against an already-confirmed wall — a live
// regression in the exact scenario the corroborator protects (review HIGH-1).
func TestTick_RealWallProbesExactlyOnceAcrossTicks(t *testing.T) {
	pane := "You have reached your usage limit. Resets in 4h.\n"
	probe := &corroboratorProbeRunner{rc: 1}
	deps := Deps{Tmux: &fakeTmux{paneSeq: []string{pane}}, Sleep: func(time.Duration) {},
		LookupEnv:       mapLookup(nil),
		CorroborateWall: DefaultWallCorroborator(probe.run, io.Discard)}.withDefaults()
	ar := newAutoResponder("claude-tmux", t.TempDir(), deps, false, 0)
	ar.prompts = nil
	ar.exhaustedRegex = c672ExhaustedPattern
	ar.injectedPrompt = "Instructions: do the task."

	var last int
	for i := 0; i < exhaustionPersistObservations+4; i++ {
		_, last = ar.tick(context.Background(), "s")
	}
	if last != 85 {
		t.Fatalf("corroborated wall must keep escalating rc 85 on every post-cross tick, got %d", last)
	}
	if len(probe.calls) != 1 {
		t.Fatalf("a confirmed wall must never re-probe: want 1 probe, got %d", len(probe.calls))
	}
}

// checkpointWallState is the stop-review site's decision (extracted for
// direct testability — review HIGH-2). Same contract as the responder:
// exactly one probe per phase, suppression logged once, a confirmed wall
// escalates on every subsequent crossing without re-probing.
func TestCheckpointWallState_HealthySuppressesOnceThenSilent(t *testing.T) {
	t.Parallel()
	probe := &corroboratorProbeRunner{rc: 0}
	st := &checkpointWallState{}
	c := DefaultWallCorroborator(probe.run, io.Discard)
	esc, sup := st.decide(context.Background(), c, "claude-tmux", true)
	if esc || !sup {
		t.Fatalf("healthy first crossing must suppress loudly: escalate=%v suppressNow=%v", esc, sup)
	}
	for i := 0; i < 3; i++ {
		if esc, sup = st.decide(context.Background(), c, "claude-tmux", true); esc || sup {
			t.Fatalf("post-suppression crossings must be silent no-ops: escalate=%v suppressNow=%v", esc, sup)
		}
	}
	if len(probe.calls) != 1 {
		t.Fatalf("exactly one probe, got %d", len(probe.calls))
	}
}

func TestCheckpointWallState_ConfirmedWallEscalatesWithoutReprobe(t *testing.T) {
	t.Parallel()
	probe := &corroboratorProbeRunner{rc: 1}
	st := &checkpointWallState{}
	c := DefaultWallCorroborator(probe.run, io.Discard)
	for i := 0; i < 3; i++ {
		esc, sup := st.decide(context.Background(), c, "claude-tmux", true)
		if !esc || sup {
			t.Fatalf("crossing %d: confirmed wall must escalate silently: escalate=%v suppressNow=%v", i, esc, sup)
		}
	}
	if len(probe.calls) != 1 {
		t.Fatalf("a confirmed wall must never re-probe: got %d probes", len(probe.calls))
	}
	if esc, _ := st.decide(context.Background(), c, "claude-tmux", false); esc {
		t.Fatal("no gate crossing ⇒ no escalation, whatever the latch holds")
	}
}
