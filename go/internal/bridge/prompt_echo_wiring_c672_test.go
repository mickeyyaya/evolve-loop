package bridge

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

func TestC672_003_TickEchoedExhaustionDoesNotEscalate(t *testing.T) {
	pane := "thinking...\n" + c672PromptEchoLine + "\nwriting report...\n"
	ar := newC672TickResponder(t, pane)
	ar.injectedPrompt = "Instructions.\n" + c672PromptEchoLine + "\nProceed."

	_, rc := ar.tick(context.Background(), "s")
	if rc == 85 {
		t.Errorf("tick() escalated rc 85 on a pane whose exhaustion text is a verbatim echo of the injected prompt — stripPromptEchoLines is not wired ahead of the exhaustion scan")
	}
}

// TestC672_004_TickGenuineExhaustionStillEscalates is the negative guard: a
// genuine CLI quota banner ABSENT from the injected prompt must still
// escalate — the echo-veto wiring must not blanket-disable the wall.
func TestC672_004_TickGenuineExhaustionStillEscalates(t *testing.T) {
	pane := "You have reached your usage limit. Resets in 4h.\n"
	ar := newC672TickResponder(t, pane)
	ar.injectedPrompt = "Instructions: do the task and report." // banner is NOT a substring

	// The genuine banner survives prompt-echo stripping, but the persistence
	// guard requires it to persist for exhaustionPersistObservations consecutive
	// ticks before escalating, so the loop drives that many ticks.
	var rc int
	for i := 0; i < exhaustionPersistObservations; i++ {
		_, rc = ar.tick(context.Background(), "s")
	}
	if rc != 85 {
		t.Errorf("tick() rc = %d after %d persistent ticks, want 85 — genuine exhaustion must survive stripping and escalate once it persists", rc, exhaustionPersistObservations)
	}
}

// TestC672_005_ResponderConstructionSitesCarryInjectedPrompt is a
// source-reader anti-gaming check: both production construction sites of
// autoResponder must reference injectedPrompt, or the behavioral tests above
// could pass on a field production never populates.
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
