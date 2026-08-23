package audit

// envexclusive_bridge_test.go — internal/bridge's integration tier must not run
// inside the loop's contended runtime, and the skip must state its REAL backstop.
//
// The evidence (2026-08-23, closing the last decided residual of the 1539-1546
// streak): the requireTmux-guarded tests boot real tmux sessions, and under a
// live wave the host's session churn makes those boots time out — 13 offenders
// on cycle-1543, every one exit=80, the same tests 7/7 PASS in 17.2s on a quiet
// host (3.6x-7.7x slower under load). That is the identical class that put
// internal/core, cmd/evolve and internal/phases/ship on the env-exclusive list
// (cycles 930/931/932).
//
// One honesty wrinkle the existing entries do not have: for those three, "CI is
// the backstop" is TRUE — CI runs their integration tests isolated. For
// internal/bridge's requireTmux subset it is FALSE: GitHub runners have no
// tmux, so requireTmux SKIPS there (the #483 finding). The skip message must
// therefore never claim CI covers what CI provably skips — a gate message that
// asserts false coverage teaches operators the same distrust a false-RED does.

import (
	"context"
	"strings"
	"testing"
)

func TestEnvExclusive_BridgeIsExcluded(t *testing.T) {
	for _, p := range []string{"./internal/bridge/...", "internal/bridge", "github.com/mickeyyaya/evolve-loop/go/internal/bridge"} {
		if !envExclusivePkg(p) {
			t.Fatalf("%q must be env-exclusive: its requireTmux tests false-RED under a live wave (cycle-1539/1543/1546 class)", p)
		}
	}
}

func TestEnvExclusive_ExistingEntriesUnchanged(t *testing.T) {
	for _, p := range []string{"internal/core", "cmd/evolve", "internal/phases/ship"} {
		if !envExclusivePkg(p) {
			t.Fatalf("%q must stay env-exclusive", p)
		}
	}
	for _, p := range []string{"internal/acssuite", "internal/prompts", "internal/inboxmover"} {
		if envExclusivePkg(p) {
			t.Fatalf("%q must NOT be env-exclusive — the list is a scalpel, not a shotgun", p)
		}
	}
}

// The backstop claim must be per-package-honest: bridge's note must say the
// requireTmux subset runs only on a quiet host, and must NOT say CI covers it.
func TestEnvExclusive_BackstopNoteIsHonestForBridge(t *testing.T) {
	note := envExclusiveBackstopNote([]string{"./internal/bridge/..."})
	if strings.Contains(note, "CI's isolated integration-tier step covers internal/bridge") {
		t.Fatalf("the note must never claim CI covers bridge — requireTmux skips there; got %q", note)
	}
	if !strings.Contains(note, "NOT covered by CI") || !strings.Contains(note, "quiet-host") {
		t.Fatalf("the note must deny CI coverage and name the quiet-host backstop; got %q", note)
	}
	// The classic entries keep their true claim — asserted on the AFFIRMATIVE
	// phrase, not a bare "CI" substring: the quiet-host denial text also
	// contains "CI", so the loose check passed against a mutant that routed
	// every package to the quiet-host claim.
	core := envExclusiveBackstopNote([]string{"./internal/core/..."})
	if !strings.Contains(core, "CI's isolated integration-tier step covers internal/core") {
		t.Fatalf("internal/core's backstop IS CI and must be affirmed, not merely mentioned; got %q", core)
	}
	if strings.Contains(core, "quiet-host") {
		t.Fatalf("internal/core must not carry the quiet-host claim; got %q", core)
	}
}

// THE WIRING (eighth of the week): the note must reach the EMITTED error from
// integrationTierScope — the string an operator and the WARN diagnostic
// actually see — not merely exist as a correct helper.
func TestIntegrationTierScope_ErrorCarriesTheHonestBackstop(t *testing.T) {
	_, err := integrationTierScope(context.Background(), nil, "", []string{"./internal/bridge/..."})
	if err == nil {
		t.Fatalf("bridge-only scope must return the env-exclusive WARN error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "quiet-host") || !strings.Contains(msg, "NOT covered by CI") {
		t.Fatalf("the emitted skip must carry bridge's real backstop; got %q", msg)
	}
	if strings.Contains(msg, "CI's isolated integration-tier step covers internal/bridge") {
		t.Fatalf("the emitted skip must not claim CI covers bridge; got %q", msg)
	}
}
