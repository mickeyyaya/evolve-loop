package opscmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func installDoctorUsage(t *testing.T, ev usageprobe.Evidence) *[]string {
	t.Helper()
	prev := doctorUsageEvidenceFn
	var asked []string
	doctorUsageEvidenceFn = func(string, bridge.Deps) (usageevidence.Explain, func()) {
		return func(_ context.Context, driver string, _ time.Time) usageprobe.Evidence {
			asked = append(asked, driver)
			return ev
		}, func() {}
	}
	t.Cleanup(func() { doctorUsageEvidenceFn = prev })
	return &asked
}

func TestDoctorLive_AFailedProbePrintsTheUsageVerdictAndItsWindows(t *testing.T) {
	window := quotastate.UsageWindow{Scope: "CLAUDE AND GPT MODELS", Kind: quotastate.KindFiveHour, PercentUsed: 100, ResetsText: "2h 15m", Family: "agy-claude", Exhausted: true}
	for verdict, want := range map[usageprobe.Verdict]string{
		usageprobe.VerdictExhausted:   "quota exhausted",
		usageprobe.VerdictHealthy:     "quota ruled out",
		usageprobe.VerdictUnavailable: "auth, install or network",
	} {
		asked := installDoctorUsage(t, usageprobe.Evidence{CLI: "agy", Family: "agy-claude", Verdict: verdict, Detail: "d", Windows: []quotastate.UsageWindow{window}})
		var errb bytes.Buffer
		_, deps := neverReadyPaneDeps(&errb)

		code := runDoctorLiveWith([]string{"agy-claude-tmux"}, io.Discard, deps)

		if code != 1 || len(*asked) != 1 || (*asked)[0] != "agy-claude-tmux" {
			t.Fatalf("%s: exit %d, asked %v", verdict, code, *asked)
		}
		for _, line := range []string{"[doctor] usage: ", want, "[doctor]   CLAUDE AND GPT MODELS 5h window 100% used, resets 2h 15m (exhausted)"} {
			if !strings.Contains(errb.String(), line) {
				t.Errorf("%s: stderr lacks %q:\n%s", verdict, line, errb.String())
			}
		}
	}
}

func TestDoctorLive_TheJSONReportCarriesTheUsageEvidenceAndAHealthyProbeAsksNothing(t *testing.T) {
	installDoctorUsage(t, usageprobe.Evidence{CLI: "claude", Family: "claude", Verdict: usageprobe.VerdictHealthy, Detail: "d"})
	var out bytes.Buffer
	_, deps := neverReadyPaneDeps(io.Discard)

	runDoctorLiveWith([]string{"claude-tmux", "--json"}, &out, deps)

	var rep struct {
		Usage *usageprobe.Evidence `json:"usage"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil || rep.Usage == nil || rep.Usage.Verdict != usageprobe.VerdictHealthy {
		t.Fatalf("json %s (err %v); want the usage evidence", out.String(), err)
	}
	var errb bytes.Buffer
	if code := reportDoctorLiveResult(liveOutcome{target: liveProbeTarget{driver: "claude-tmux"}, rc: bridge.ExitOK}, io.Discard, &errb); code != 0 || strings.Contains(errb.String(), "usage") {
		t.Errorf("a healthy probe printed usage: %q", errb.String())
	}
}
