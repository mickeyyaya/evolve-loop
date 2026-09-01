package audit

// envexclusive_bridge_test.go — the env-exclusive list is a SCALPEL: exactly
// internal/bridge, whose requireTmux tests boot real tmux sessions that time
// out under a live wave (13 offenders on cycle-1543, all exit=80; 7/7 PASS in
// 17.2s on a quiet host) AND skip in CI (no tmux on runners — the #483
// finding), so neither the serialized retake nor CI can vouch for it. Its
// honest backstop is a quiet-host run, and the skip message must say so.
//
// Everything else runs in the tier. internal/core, cmd/evolve and
// internal/phases/ship were excluded 2026-07-19 for contention false-REDs
// (cycles 930/931/932) — one day before the serialized retake-on-red
// (3c5ed711) cured that exact problem — and the stale exclusion shipped
// cycle-1594's red to main (2.5 days CI-red; #519). This file's earlier
// revision PINNED the fossil ("must stay env-exclusive"); the pins now point
// the other way, with the retake as the contention answer.

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

func TestEnvExclusive_OnlyBridge(t *testing.T) {
	for _, p := range []string{"internal/core", "cmd/evolve", "internal/phases/ship",
		"internal/acssuite", "internal/prompts", "internal/inboxmover"} {
		if envExclusivePkg(p) {
			t.Fatalf("%q must NOT be env-exclusive — the list is a scalpel (bridge only); contention is the serialized retake's job, and the last fossil here cost 2.5 days of red main (cycle-1594)", p)
		}
	}
}

// The backstop claim must be per-package-honest: bridge's note must say the
// requireTmux subset runs only on a quiet host, and must NOT say CI covers it.
func TestEnvExclusive_BackstopNoteIsHonestForBridge(t *testing.T) {
	note := envExclusiveBackstopNote([]string{"./internal/bridge/..."})
	// Reintroduction guard: no production code can emit this phrase today
	// (the CI-affirmative renderer branch was deleted 2026-09-01); the pin
	// exists so a future renderer cannot bring the false claim back.
	if strings.Contains(note, "CI's isolated integration-tier step covers internal/bridge") {
		t.Fatalf("the note must never claim CI covers bridge — requireTmux skips there; got %q", note)
	}
	if !strings.Contains(note, envExclusiveNoCIMarker) || !strings.Contains(note, "quiet-host") {
		t.Fatalf("the note must deny CI coverage and name the quiet-host backstop; got %q", note)
	}
	if !strings.Contains(note, "requireTmux tests boot real tmux sessions") {
		t.Fatalf("the note must carry the entry's evidence (why), not only the backstop; got %q", note)
	}
}

// TestEnvExclusive_EntriesDeclareNoCIBackstop is the RULE, expressed over the
// record table so it needs no package names and cannot fossilize (2026-09-01
// architecture review): a package may be env-exclusive ONLY when CI provides
// no backstop — a CI-covered package belongs IN the lane tier (cycle-1594).
// Every entry must carry its evidence and an honest backstop.
func TestEnvExclusive_EntriesDeclareNoCIBackstop(t *testing.T) {
	if len(integrationTierEnvExclusive) == 0 {
		t.Fatal("the record table is empty — if the last exclusion was removed on purpose, retire this guard deliberately, not by vacuity")
	}
	for _, e := range integrationTierEnvExclusive {
		if e.pkg == "" || e.why == "" || e.backstop == "" {
			t.Fatalf("entry %+v must carry pkg, evidence, and backstop — an unexplained exclusion is the fossil class", e)
		}
		if !strings.Contains(e.backstop, envExclusiveNoCIMarker) {
			t.Fatalf("%s: an env-exclusive entry must declare CI does NOT back it — a CI-covered package belongs in the lane tier (cycle-1594); backstop: %q", e.pkg, e.backstop)
		}
		// Reintroduction guard (phrase unfireable by design since 2026-09-01).
		if strings.Contains(e.backstop, "CI's isolated integration-tier step covers") {
			t.Fatalf("%s: backstop claims CI coverage — contradiction with the selection criterion; backstop: %q", e.pkg, e.backstop)
		}
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

// TestIntegrationTierScope_CoversCoreCmdShip_Cycle1594 is the INSTANCE
// regression for the incident — the RULE lives in
// TestEnvExclusive_EntriesDeclareNoCIBackstop, expressed over the record
// table. This pin names the three packages whose fossilized skip (2026-07-19,
// superseded one day later by the serialized retake, 3c5ed711) cost 2.5 days
// of red main on cycle-1594 (20e839ee; #519; #518): they specifically must
// stay in the lane tier scope even if someone re-lists one with a
// rule-satisfying but false backstop claim.
func TestIntegrationTierScope_CoversCoreCmdShip_Cycle1594(t *testing.T) {
	changed := []string{"./internal/core/...", "./cmd/evolve/...", "./internal/phases/ship/..."}
	pkgs, err := integrationTierScope(context.Background(), nil, "", changed)
	if err != nil {
		t.Fatalf("CI-backstopped packages must not be skipped as env-exclusive (cycle-1594 class): %v", err)
	}
	want := map[string]bool{}
	for _, p := range pkgs {
		want[p] = true
	}
	for _, p := range changed {
		if !want[p] {
			t.Fatalf("%q missing from the lane integration-tier scope; got %v", p, pkgs)
		}
	}
}
