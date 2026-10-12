package runnertest

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable/gatesignal"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestFreeSleep_ReturnsAtOnce(t *testing.T) {
	start := time.Now()
	FreeSleep(time.Hour)
	if time.Since(start) > time.Second {
		t.Fatal("FreeSleep waited")
	}
}

func TestEmptyArtifactVerifier_ReportsEmptyArtifactForTheDispatchedPath(t *testing.T) {
	res, err := EmptyArtifactVerifier().VerifyForClassification(gatesignalCheck(), "scout", rootsFor("/ws/x.md"))
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.OK || res.Phase != "scout" || res.ArtifactPath != "/ws/x.md" || len(res.Violations) != 1 || res.Violations[0].Code != "empty_artifact" {
		t.Errorf("result=%+v", res)
	}
}

func TestSleepRecorder_SleepAndCallsKeepOrderAndCopy(t *testing.T) {
	var r SleepRecorder
	r.Sleep(time.Second)
	r.Sleep(2 * time.Second)
	calls := r.Calls()
	calls[0] = 0
	if got := r.Calls(); len(got) != 2 || got[0] != time.Second || got[1] != 2*time.Second {
		t.Errorf("calls=%v", got)
	}
}

type recordingTB struct {
	testing.TB
	errors int
}

func (r *recordingTB) Helper()               {}
func (r *recordingTB) Errorf(string, ...any) { r.errors++ }

func ladderOf(n int, d time.Duration) *SleepRecorder {
	r := &SleepRecorder{}
	for i := 0; i < n; i++ {
		r.Sleep(d)
	}
	return r
}

func TestAssertProductionLadder_AcceptsOnlyFifteenTimes200ms(t *testing.T) {
	cases := []struct {
		name   string
		r      *SleepRecorder
		errors int
	}{
		{"production ladder", ladderOf(15, 200*time.Millisecond), 0},
		{"one retry short", ladderOf(14, 200*time.Millisecond), 1},
		{"interval halved", ladderOf(15, 100*time.Millisecond), 15},
		{"no sleeps", ladderOf(0, 0), 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb := &recordingTB{TB: t}
			c.r.AssertProductionLadder(tb)
			if tb.errors != c.errors {
				t.Errorf("errors=%d, want %d", tb.errors, c.errors)
			}
		})
	}
}

func gatesignalCheck() gatesignal.Check { return gatesignal.Check{} }

func rootsFor(dispatched string) phasecontract.Roots {
	return phasecontract.Roots{DispatchedArtifact: dispatched}
}
