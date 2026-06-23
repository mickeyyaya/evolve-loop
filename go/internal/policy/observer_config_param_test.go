package policy_test

// ObserverPolicy — the typed parameters that replaced EVOLVE_OBSERVER_*. Pointer
// fields exist precisely to distinguish "omitted" (nil → built-in default) from
// "explicit zero/false" (e.g. nudge_s:0 DISABLES nudging). The accessor must
// never return a nil pointer; cmd_phase_observer dereferences each directly.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/phaseobserver"
	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

type observerWant struct {
	autospawn        bool
	pollS            int
	stallS           int
	nudgeS           int
	nudgeBody        string
	eofGraceS        int
	watchdogPollS    int
	watchdogWarnPct  int
	watchdogGraceS   int
	watchdogDisabled bool
}

func assertObserver(t *testing.T, got policy.ObserverPolicy, want observerWant) {
	t.Helper()
	if v := derefBool(t, "Autospawn", got.Autospawn); v != want.autospawn {
		t.Errorf("Autospawn = %v, want %v", v, want.autospawn)
	}
	if v := derefInt(t, "PollS", got.PollS); v != want.pollS {
		t.Errorf("PollS = %d, want %d", v, want.pollS)
	}
	if v := derefInt(t, "StallS", got.StallS); v != want.stallS {
		t.Errorf("StallS = %d, want %d", v, want.stallS)
	}
	if v := derefInt(t, "NudgeS", got.NudgeS); v != want.nudgeS {
		t.Errorf("NudgeS = %d, want %d", v, want.nudgeS)
	}
	if got.NudgeBody != want.nudgeBody {
		t.Errorf("NudgeBody = %q, want %q", got.NudgeBody, want.nudgeBody)
	}
	if got.EOFGraceS != want.eofGraceS {
		t.Errorf("EOFGraceS = %d, want %d", got.EOFGraceS, want.eofGraceS)
	}
	if v := derefInt(t, "WatchdogPollS", got.WatchdogPollS); v != want.watchdogPollS {
		t.Errorf("WatchdogPollS = %d, want %d", v, want.watchdogPollS)
	}
	if v := derefInt(t, "WatchdogWarnPct", got.WatchdogWarnPct); v != want.watchdogWarnPct {
		t.Errorf("WatchdogWarnPct = %d, want %d", v, want.watchdogWarnPct)
	}
	if v := derefInt(t, "WatchdogGraceS", got.WatchdogGraceS); v != want.watchdogGraceS {
		t.Errorf("WatchdogGraceS = %d, want %d", v, want.watchdogGraceS)
	}
	if got.WatchdogDisabled != want.watchdogDisabled {
		t.Errorf("WatchdogDisabled = %v, want %v", got.WatchdogDisabled, want.watchdogDisabled)
	}
}

func defaultObserverWant() observerWant {
	return observerWant{autospawn: true, pollS: 5, stallS: 600, nudgeS: 300, watchdogPollS: 15, watchdogWarnPct: 75, watchdogGraceS: 10}
}

func TestObserverConfig_Resolution(t *testing.T) {
	withAutospawnFalse := defaultObserverWant()
	withAutospawnFalse.autospawn = false
	withNudgeDisabled := defaultObserverWant()
	withNudgeDisabled.nudgeS = 0
	withWarnPctNoClamp := defaultObserverWant()
	withWarnPctNoClamp.watchdogWarnPct = 150

	cases := []struct {
		name string
		pol  policy.Policy
		want observerWant
	}{
		{"absent-defaults", policy.Policy{}, defaultObserverWant()},
		{"empty-block-defaults", policy.Policy{Observer: &policy.ObserverPolicy{}}, defaultObserverWant()},
		{"autospawn-explicit-false", policy.Policy{Observer: &policy.ObserverPolicy{Autospawn: boolPtr(false)}}, withAutospawnFalse},
		{"nudge-explicit-zero-disables", policy.Policy{Observer: &policy.ObserverPolicy{NudgeS: intPtr(0)}}, withNudgeDisabled},
		{"watchdog-warn-pct-no-clamp", policy.Policy{Observer: &policy.ObserverPolicy{WatchdogWarnPct: intPtr(150)}}, withWarnPctNoClamp},
		{"full-override", policy.Policy{Observer: &policy.ObserverPolicy{
			Autospawn: boolPtr(false), PollS: intPtr(10), StallS: intPtr(900), NudgeS: intPtr(120),
			NudgeBody: "wake up", EOFGraceS: 3, WatchdogPollS: intPtr(20), WatchdogWarnPct: intPtr(50),
			WatchdogGraceS: intPtr(5), WatchdogDisabled: true,
		}}, observerWant{autospawn: false, pollS: 10, stallS: 900, nudgeS: 120, nudgeBody: "wake up", eofGraceS: 3, watchdogPollS: 20, watchdogWarnPct: 50, watchdogGraceS: 5, watchdogDisabled: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertObserver(t, tc.pol.ObserverConfig(), tc.want)
		})
	}
}

func TestLoad_ObserverBlock(t *testing.T) {
	// Explicit zero/false JSON values must survive (nudge_s:0 disables nudging,
	// autospawn:false disables spawn, watchdog_disabled:true).
	json := `{"observer":{"autospawn":false,"poll_s":10,"stall_s":900,"nudge_s":0,` +
		`"nudge_body":"wake up","eof_grace_s":3,"watchdog_poll_s":20,` +
		`"watchdog_warn_pct":150,"watchdog_grace_s":5,"watchdog_disabled":true}}`
	pol, err := policy.Load(writeTempPolicy(t, json))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	assertObserver(t, pol.ObserverConfig(), observerWant{
		autospawn: false, pollS: 10, stallS: 900, nudgeS: 0, nudgeBody: "wake up", eofGraceS: 3,
		watchdogPollS: 20, watchdogWarnPct: 150, watchdogGraceS: 5, watchdogDisabled: true,
	})

	// Absent observer block → all built-in defaults.
	def, err := policy.Load(writeTempPolicy(t, `{}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	assertObserver(t, def.ObserverConfig(), defaultObserverWant())
}

// TestObserverConfig_WiringToPhaseObserver documents the cmd_phase_observer
// mapping: ObserverConfig() feeds phaseobserver.Config by dereferencing each
// *int pointer. The never-nil guarantee is what makes those derefs panic-free.
func TestObserverConfig_WiringToPhaseObserver(t *testing.T) {
	oc := policy.Policy{}.ObserverConfig()
	cfg := phaseobserver.Config{
		PollS:     derefInt(t, "PollS", oc.PollS),
		StallS:    derefInt(t, "StallS", oc.StallS),
		NudgeS:    derefInt(t, "NudgeS", oc.NudgeS),
		NudgeBody: oc.NudgeBody,
		EOFGraceS: oc.EOFGraceS,
	}
	if cfg.PollS != 5 || cfg.StallS != 600 || cfg.NudgeS != 300 {
		t.Errorf("wired phaseobserver.Config = %+v, want PollS=5 StallS=600 NudgeS=300", cfg)
	}
}
