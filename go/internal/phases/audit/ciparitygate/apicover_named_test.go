package ciparitygate

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestApicoverNamed_EveryExportIsNamedAndExercised(t *testing.T) {
	for _, c := range []signalcenter.Code{CodeChangeSetUnderivable, CodeGateStepFailed, CodeGateFailed, CodeTierEnvExclusiveSkipped,
		CodeTierFlakeAbsorbed, CodeTierDeadlineNoVerdict, CodeTierRetakeExecFailed, CodeTierLockUnavailable, CodeTierLogWriteFailed, CodeGraduationDeferred,
		CodeTierBaseRedAttributed, CodeTierBaseRedUnjudged} {
		if !c.Valid() {
			t.Errorf("%s is malformed", c)
		}
	}
	var req Request = Request{1, "/p", "/w", "/ws"}
	var timeouts Timeouts = DefaultTimeouts()
	if timeouts.TierAttempt != 15*time.Minute {
		t.Fatal("DefaultTimeouts")
	}
	var set ChangedSetFunc = fixedSet("./internal/p/...")
	var opt Option = WithTimeouts(timeouts)
	var attribution Attribution = func(context.Context, baseattr.Reds) (baseattr.Verdict, error) { return baseattr.Verdict{}, nil }
	var notice error = BaseRed("red on the base")
	if notice.Error() != "red on the base" {
		t.Fatal("BaseRed")
	}
	if env := TierEnv(); len(env) == 0 {
		t.Fatal("TierEnv")
	}
	c := signalcenter.New()
	var gates *Gates = New(sysexec.RunFunc(fakeRunFunc(0, "", "", nil)), set, opt,
		WithClock(time.Now, time.Sleep), WithAttribution(attribution), WithSignals(func() *signalcenter.Center { return c }))
	if !gates.SignalsWired() {
		t.Fatal("SignalsWired")
	}
	for name, gate := range map[string]func(Request) ([]string, error){
		"GoVet": gates.GoVet, "ACSDurable": gates.ACSDurable, "IntegrationTier": gates.IntegrationTier,
		"ApicoverEnforce": gates.ApicoverEnforce, "ApicoverGraduation": gates.ApicoverGraduation,
	} {
		if off, err := gate(req); off != nil || err != nil {
			t.Errorf("%s on a module-less request = (%v, %v)", name, off, err)
		}
	}
}
