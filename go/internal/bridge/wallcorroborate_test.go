package bridge

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

func TestDefaultWallCorroborator_HungProbeIsWalled(t *testing.T) {
	// NOT parallel — this test WRITES the package-level wallProbeTimeout var;
	// Go runs serial tests to completion before any t.Parallel test resumes,
	// which is the only ordering that makes the write race-free.
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
