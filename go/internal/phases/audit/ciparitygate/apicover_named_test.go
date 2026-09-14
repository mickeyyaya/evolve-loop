package ciparitygate

// apicover_named_test.go — names and exercises every export of the unit
// (the acs/regression/apicover completeness predicate and `apicover -enforce`
// count package-local tests only).

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestApicoverNamed_EveryExportIsNamedAndExercised(t *testing.T) {
	for _, c := range []signalcenter.Code{CodeChangeSetUnderivable, CodeGateStepFailed, CodeGateFailed, CodeTierEnvExclusiveSkipped,
		CodeTierFlakeAbsorbed, CodeTierDeadlineNoVerdict, CodeTierRetakeExecFailed, CodeTierLockUnavailable, CodeTierLogWriteFailed, CodeGraduationDeferred} {
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
	c := signalcenter.New()
	var gates *Gates = New(sysexec.RunFunc(fakeRunFunc(0, "", "", nil)), set, opt,
		WithClock(time.Now, time.Sleep), WithSignals(func() *signalcenter.Center { return c }))
	if !gates.SignalsWired() {
		t.Fatal("SignalsWired")
	}
	for name, gate := range map[string]func(Request) ([]string, error){
		"GoVet": gates.GoVet, "ACSDurable": gates.ACSDurable, "IntegrationTier": gates.IntegrationTier,
		"ApicoverEnforce": gates.ApicoverEnforce, "ApicoverGraduation": gates.ApicoverGraduation,
	} {
		if off, err := gate(req); off != nil || err != nil { // no go.mod under /w → every gate is a silent no-op
			t.Errorf("%s on a module-less request = (%v, %v)", name, off, err)
		}
	}
}
