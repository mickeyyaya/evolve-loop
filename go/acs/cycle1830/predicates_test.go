//go:build acs

package cycle1830

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestC1830_001_PhaseCycleFlagRunsScoutWithTheCyclesOwnState(t *testing.T) {
	fx := newCycleFixture(t)
	scout := registerRecordingScout(t)

	r := runPhaseCommand("", append([]string{"scout"}, fx.cycleFlags()...)...)
	if r.code != exitOK {
		t.Fatalf("evolve phase scout --cycle %d must run from %s:\n%s", fixtureCycle, fx.statePath(), r)
	}
	if len(scout.dispatched) != 1 {
		t.Fatalf("scout dispatched %d times, want exactly once:\n%s", len(scout.dispatched), r)
	}
	assertDerivedFromState(t, scout.dispatched[0], fx)
	var resp core.PhaseResponse
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil || resp.Verdict != core.VerdictPASS {
		t.Errorf("stdout must carry the scout response (verdict PASS); parse err=%v:\n%s", err, r)
	}
}

func TestC1830_002_PhaseCycleFlagRefusesACycleItCannotSafelyRerun(t *testing.T) {
	for _, tc := range derivationRefusals() {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCycleFixture(t)
			tc.arrange(t, fx)
			scout := registerRecordingScout(t)

			r := runPhaseCommand("", append([]string{"scout"}, fx.cycleFlags()...)...)
			if r.code != exitDerivationRefused {
				t.Errorf("want exit %d for %s:\n%s", exitDerivationRefused, tc.name, r)
			}
			if !namesAny(r.stderr, tc.reasons(fx)) {
				t.Errorf("stderr must name the reason (any of %q):\n%s", tc.reasons(fx), r)
			}
			if len(scout.dispatched) != 0 {
				t.Errorf("a refused cycle must dispatch no phase; scout ran %d time(s)", len(scout.dispatched))
			}
		})
	}
	t.Run("a stale lease no longer holds the cycle", func(t *testing.T) {
		fx := newCycleFixture(t)
		longExpiredHeartbeat := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
		fx.holdLease(t, longExpiredHeartbeat)
		scout := registerRecordingScout(t)

		r := runPhaseCommand("", append([]string{"scout"}, fx.cycleFlags()...)...)
		if r.code != exitOK || len(scout.dispatched) != 1 {
			t.Fatalf("a lease whose heartbeat is past its TTL must not refuse the rerun (dispatched=%d):\n%s", len(scout.dispatched), r)
		}
		assertDerivedFromState(t, scout.dispatched[0], fx)
	})
}

func TestC1830_003_PhaseCycleFlagExits10OnAStdinRequestOrAMalformedCycle(t *testing.T) {
	stdinConflicts := []struct {
		name    string
		stdin   string
		arrange func(t *testing.T, fx cycleFixture)
	}{
		{"a request for the same cycle", `{"cycle":4242}`, nil},
		{"a request naming another workspace", `{"cycle":7,"workspace":"/elsewhere","worktree":"/elsewhere/wt"}`, nil},
		{"a request while the cycle has no state", `{"cycle":4242}`, func(t *testing.T, fx cycleFixture) { removeOrFail(t, fx.statePath()) }},
	}
	for _, tc := range stdinConflicts {
		t.Run("stdin carries "+tc.name, func(t *testing.T) {
			fx := newCycleFixture(t)
			if tc.arrange != nil {
				tc.arrange(t, fx)
			}
			scout := registerRecordingScout(t)

			r := runPhaseCommand(tc.stdin, append([]string{"scout"}, fx.cycleFlags()...)...)
			if r.code != exitUsage {
				t.Errorf("--cycle together with a stdin request must exit %d:\n%s", exitUsage, r)
			}
			if !strings.Contains(strings.ToLower(r.stderr), "stdin") {
				t.Errorf("stderr must name the stdin conflict:\n%s", r)
			}
			if len(scout.dispatched) != 0 {
				t.Errorf("a conflicting invocation must dispatch no phase; scout ran %d time(s)", len(scout.dispatched))
			}
		})
	}
	t.Run("whitespace-only stdin is no request", func(t *testing.T) {
		fx := newCycleFixture(t)
		scout := registerRecordingScout(t)

		r := runPhaseCommand("\n  \t\n", append([]string{"scout"}, fx.cycleFlags()...)...)
		if r.code != exitOK || len(scout.dispatched) != 1 {
			t.Fatalf("blank stdin beside --cycle must run the derived request (dispatched=%d):\n%s", len(scout.dispatched), r)
		}
		assertDerivedFromState(t, scout.dispatched[0], fx)
	})
	t.Run("a malformed --cycle value", func(t *testing.T) {
		fx := newCycleFixture(t)
		scout := registerRecordingScout(t)

		r := runPhaseCommand("", "scout", "--cycle", "not-a-number", "--project-root", fx.root)
		if r.code != exitUsage || len(scout.dispatched) != 0 {
			t.Errorf("a non-integer --cycle must exit %d and dispatch nothing (dispatched=%d):\n%s", exitUsage, len(scout.dispatched), r)
		}
	})
}

func TestC1830_004_PhaseWithoutCycleStillReadsTheStdinRequest(t *testing.T) {
	t.Run("a stdin request is dispatched as given", func(t *testing.T) {
		scout := registerRecordingScout(t)
		stdinRequest := core.PhaseRequest{Cycle: 77, ProjectRoot: "/stdin/root", Workspace: "/stdin/ws", Worktree: "/stdin/wt", GoalHash: "stdin-goal"}
		body, err := json.Marshal(stdinRequest)
		if err != nil {
			t.Fatal(err)
		}

		r := runPhaseCommand(string(body), "scout")
		if r.code != exitOK || len(scout.dispatched) != 1 {
			t.Fatalf("evolve phase scout without --cycle must run the stdin request (dispatched=%d):\n%s", len(scout.dispatched), r)
		}
		got := scout.dispatched[0]
		if got.Cycle != 77 || got.Workspace != "/stdin/ws" || got.Worktree != "/stdin/wt" || got.GoalHash != "stdin-goal" {
			t.Errorf("dispatched %+v, want the stdin request %+v", got, stdinRequest)
		}
	})
	t.Run("malformed stdin JSON still exits 11", func(t *testing.T) {
		scout := registerRecordingScout(t)

		r := runPhaseCommand("not-json", "scout")
		if r.code != exitStdinDecode || len(scout.dispatched) != 0 {
			t.Errorf("want exit %d and no dispatch (dispatched=%d):\n%s", exitStdinDecode, len(scout.dispatched), r)
		}
	})
}

func TestC1830_005_ComposeCycleFlagDerivesTheRequestThroughTheEvolveBinary(t *testing.T) {
	t.Run("a dry run plans scout then triage for the derived cycle", func(t *testing.T) {
		fx := newCycleFixture(t)

		r := runEvolve(t, fx, "", composeDryRun(fx)...)
		if r.code != exitOK {
			t.Fatalf("evolve compose --phases scout,triage --cycle %d --dry-run must exit 0:\n%s", fixtureCycle, r)
		}
		if !strings.Contains(r.stdout, "scout -> triage") || !strings.Contains(r.stdout, "DRY-RUN") {
			t.Errorf("stdout must plan scout -> triage as a dry run:\n%s", r)
		}
	})
	for _, tc := range derivationRefusals() {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			fx := newCycleFixture(t)
			tc.arrange(t, fx)

			r := runEvolve(t, fx, "", composeDryRun(fx)...)
			if r.code != exitDerivationRefused {
				t.Errorf("want exit %d for %s, even on a dry run:\n%s", exitDerivationRefused, tc.name, r)
			}
			if !namesAny(r.stderr, tc.reasons(fx)) {
				t.Errorf("stderr must name the reason (any of %q):\n%s", tc.reasons(fx), r)
			}
		})
	}
	t.Run("refused: a stdin request beside --cycle", func(t *testing.T) {
		fx := newCycleFixture(t)

		r := runEvolve(t, fx, `{"cycle":4242}`, composeDryRun(fx)...)
		if r.code != exitUsage {
			t.Errorf("want exit %d:\n%s", exitUsage, r)
		}
		if strings.Contains(r.stderr, "flag provided but not defined") || !strings.Contains(strings.ToLower(r.stderr), "stdin") {
			t.Errorf("the exit must come from the stdin conflict, not from an undefined flag:\n%s", r)
		}
	})
}
