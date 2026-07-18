package bridge

// RED wiring tests for cycle-672 top_n task `echo-veto-wiring-completion`,
// AC2: stripPromptEchoLines (landed cycle-654, TestC654_004 green) has ZERO
// production call sites — autoResponder.tick() still hands the RAW pane to the
// exhaustion scan (autorespond.go, ExhaustedOf) and to decideAutoRespond, so
// an agent echoing its own Deliverable-Contract exhaustion instructions can
// escalate rc 85 / classify rate_limit (cycle-656 retro D3 fired live).
//
// The fix (scout map): add an injectedPrompt field to autoResponder, populate
// it from the already-resolved prompt at BOTH construction sites
// (driver_tmux_repl.go, recipe_adapter.go), and strip the pane via
// stripPromptEchoLines in tick() ahead of the exhaustion check.
//
// RED today: autoResponder has no injectedPrompt field — compile failure.
// DO NOT MODIFY THESE TESTS (echo-veto intent). C672_004 is the negative guard
// (genuine banner must STILL escalate) and must be GREEN after the wiring lands.
// C672_005 is the discriminating anti-gaming check for the construction-site
// half — the behavioral tests alone could be satisfied by a field nothing
// populates in production (exactly the class of gap that let cycles 654/656 slip).
//
// UPDATE (exhaustion-gate, 2026-07): C672_004's tick COUNT was revised from 1 to
// exhaustionPersistObservations because the exhaustion fast-fail is now
// persistence-gated (exhaustion_gate.go) — a genuine wall still escalates (intent
// preserved: it survives prompt-echo stripping), it just does so on the
// threshold-th consecutive tick, not the first. The echo-veto behavior C672_003
// and C672_005 pin is unchanged.

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const c672ExhaustedPattern = `(?i)reached your usage limit`

// c672PromptEchoLine is the agent's OWN instruction line: present verbatim in
// the injected prompt AND echoed to the pane.
const c672PromptEchoLine = "Deliverable-Contract: If you have reached your usage limit, stop and hand off."

// newC672TickResponder builds a tick-ready responder over a scripted pane with
// a controlled exhaustion pattern and no auto-respond rules.
func newC672TickResponder(t *testing.T, pane string) *autoResponder {
	t.Helper()
	deps := Deps{Tmux: &fakeTmux{paneSeq: []string{pane}}, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
	ar := newAutoResponder("claude-tmux", t.TempDir(), deps, false, 0)
	ar.prompts = nil
	ar.exhaustedRegex = c672ExhaustedPattern
	return ar
}

// TestC672_003_TickEchoedExhaustionDoesNotEscalate — AC2 (positive): a pane
// whose only exhaustion-matching text is a verbatim echo of the injected
// prompt must NOT escalate rc 85 out of tick().
func TestC672_003_TickEchoedExhaustionDoesNotEscalate(t *testing.T) {
	pane := "thinking...\n" + c672PromptEchoLine + "\nwriting report...\n"
	ar := newC672TickResponder(t, pane)
	ar.injectedPrompt = "Instructions.\n" + c672PromptEchoLine + "\nProceed." // RED: field does not exist yet

	_, rc := ar.tick(context.Background(), "s")
	if rc == 85 {
		t.Errorf("tick() escalated rc 85 on a pane whose exhaustion text is a verbatim echo of the injected prompt — stripPromptEchoLines is not wired ahead of the exhaustion scan")
	}
}

// TestC672_004_TickGenuineExhaustionStillEscalates — AC2/AC3 (negative guard):
// a genuine CLI quota banner ABSENT from the injected prompt must still
// escalate rc 85 — the echo-veto wiring must not blanket-disable the wall.
func TestC672_004_TickGenuineExhaustionStillEscalates(t *testing.T) {
	pane := "You have reached your usage limit. Resets in 4h.\n"
	ar := newC672TickResponder(t, pane)
	ar.injectedPrompt = "Instructions: do the task and report." // banner is NOT a substring

	// The genuine banner survives prompt-echo stripping (walled every tick), but
	// the persistence guard (exhaustion_gate.go) requires it to PERSIST for
	// exhaustionPersistObservations consecutive ticks before escalating — the fake
	// pane replays the banner every capture, so a genuine wall crosses on the
	// threshold-th tick. A single transient frame (wall text passing through a
	// working agent's pane) would never cross — that is the guard's purpose.
	var rc int
	for i := 0; i < exhaustionPersistObservations; i++ {
		_, rc = ar.tick(context.Background(), "s")
	}
	if rc != 85 {
		t.Errorf("tick() rc = %d after %d persistent ticks, want 85 — genuine exhaustion must survive stripping and escalate once it persists", rc, exhaustionPersistObservations)
	}
}

// TestC672_005_ResponderConstructionSitesCarryInjectedPrompt — AC2
// (construction wiring, discriminating anti-gaming supplement to the
// behavioral pair above): both production construction sites of
// autoResponder must reference injectedPrompt, i.e. populate the field from
// their already-resolved prompt. Without this, the behavioral tests pass on a
// field production never sets — the exact helper-without-wiring gap this task
// closes. Load-bearing behavior is asserted by C672_003/004; this only pins
// that the wiring reaches BOTH launch surfaces.
func TestC672_005_ResponderConstructionSitesCarryInjectedPrompt(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve this test file's path via runtime.Caller")
	}
	dir := filepath.Dir(thisFile)
	for _, f := range []string{"driver_tmux_repl.go", "recipe_adapter.go"} {
		src, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if !strings.Contains(string(src), "injectedPrompt") {
			t.Errorf("%s never references injectedPrompt — this autoResponder construction site does not thread the resolved prompt into the echo-veto", f)
		}
	}
}
