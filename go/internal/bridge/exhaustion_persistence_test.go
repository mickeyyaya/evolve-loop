package bridge

import (
	"context"
	"testing"
	"time"
)

// A working agent that momentarily RENDERS wall-shaped text — a cat/grep/diff of
// a file, test fixture, or incident report that quotes a provider's "reached your
// … limit" message — must NOT be fast-failed. The text is gone by the next
// observation as the agent's fresh output replaces it, so the persistence gate
// never crosses. A genuine wall (the CLI parked, present every frame) DOES cross.
//
// This is the raw-pane false-FAIL class the two-round go-review of the per-model
// exhausted_regex surfaced: killing a working agent (exit 85 → cross-family
// failover) is the cardinal sin (cycle-254/255/314/641), strictly worse than
// missing a wall (which merely fails over). The regex tightening reduced the
// match surface; THIS persistence guard is the durable class fix.
func TestExhaustion_TransientWallTextDoesNotFastFail(t *testing.T) {
	const wallRegex = `(?i)reached your usage limit`
	wall := "You've reached your usage limit. Run /usage-credits.\n"
	fresh := "> Reading incident-report.md ... done. Now writing the audit.\n"

	newAR := func(t *testing.T, seq []string) *autoResponder {
		t.Helper()
		deps := Deps{Tmux: &fakeTmux{paneSeq: seq}, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
		ar := newAutoResponder("claude-tmux", t.TempDir(), deps, false, 0)
		ar.prompts = nil
		ar.exhaustedRegex = wallRegex
		return ar
	}

	t.Run("transient wall text (agent reads a file, then continues) never fast-fails", func(t *testing.T) {
		// tick 1: wall text visible; tick 2+: fresh agent output (fakeTmux replays
		// the LAST paneSeq entry), so the wall is gone and the streak resets.
		ar := newAR(t, []string{wall, fresh})
		for i := 0; i < 5; i++ {
			if _, rc := ar.tick(context.Background(), "s"); rc == 85 {
				t.Fatalf("tick #%d fast-failed (rc 85) on a TRANSIENT wall-text frame — a WORKING agent was killed (cardinal false-FAIL)", i)
			}
		}
	})

	t.Run("persistent wall (CLI parked) still fast-fails once it crosses the threshold", func(t *testing.T) {
		ar := newAR(t, []string{wall}) // replays every capture — the CLI is parked at the wall
		fired := false
		for i := 0; i < exhaustionPersistObservations+2 && !fired; i++ {
			if _, rc := ar.tick(context.Background(), "s"); rc == 85 {
				fired = true
			}
		}
		if !fired {
			t.Fatalf("a persistent wall did not fast-fail within %d ticks — the exit-85 fallback path is broken", exhaustionPersistObservations+2)
		}
	})
}
