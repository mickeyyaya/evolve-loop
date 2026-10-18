package cliupdate_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func explainedUpdate(t *testing.T, probeErr error, verdict usageprobe.Verdict) (cliupdate.Result, []string) {
	t.Helper()
	h := &fakeHost{
		versions: map[string][]string{"agy": {"1.3.0"}},
		probeErr: map[string]error{"agy": probeErr},
	}
	var asked []string
	seams := h.seams()
	seams.Explain = func(_ context.Context, family string) usageprobe.Evidence {
		asked = append(asked, family)
		return usageprobe.Evidence{CLI: "agy", Family: family, Verdict: verdict, Detail: "GEMINI MODELS week window 100% used, resets 2h"}
	}
	agy := cliupdate.Family{Name: "agy", UpdateArgv: []string{"agy", "update"}}
	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{agy}, seams, nil))
	if h.called("run") || h.called("smoke") {
		t.Errorf("a family whose probe failed was updated or smoke-tested: %q", h.calls)
	}
	return res, asked
}

var bootTimeout = fmt.Errorf("doctor live agy-tmux: %w after 2 attempt(s); final pane: signing in", cliupdate.ErrBootTimeout)

func TestUpdate_ABootTimeoutWithHealthyUsageIsABootTimeoutNotUnsubscribedNorQuota(t *testing.T) {
	res, asked := explainedUpdate(t, bootTimeout, usageprobe.VerdictHealthy)

	if res.Status != cliupdate.StatusBootTimeout || len(asked) != 1 || asked[0] != "agy" {
		t.Fatalf("result %+v after asking %v; want boot-timeout after one usage query", res, asked)
	}
	for _, banned := range []string{"unsubscribed", "quota exhausted"} {
		if strings.Contains(res.Detail, banned) {
			t.Errorf("detail %q says %q", res.Detail, banned)
		}
	}
	if !strings.Contains(res.Detail, "quota ruled out") || !strings.Contains(res.Detail, "signing in") {
		t.Errorf("detail %q must carry the ruling-out evidence and the pane", res.Detail)
	}
}

func TestUpdate_AnyProbeFailureWithAnExhaustedWindowIsAVerifiedQuotaCause(t *testing.T) {
	for name, probeErr := range map[string]error{
		"boot timeout": bootTimeout,
		"walled":       errors.New("doctor live agy-tmux rc=85 pattern=\"rate_limit\""),
	} {
		res, _ := explainedUpdate(t, probeErr, usageprobe.VerdictExhausted)

		if res.Status != cliupdate.StatusQuotaExhausted || !strings.Contains(res.Detail, "quota exhausted") || !strings.Contains(res.Detail, "GEMINI MODELS") {
			t.Errorf("%s: result %+v; want quota-exhausted naming the drained window", name, res)
		}
		if !strings.HasPrefix(res.Line(), "agy quota-exhausted 1.3.0: ") {
			t.Errorf("%s: Line() = %q", name, res.Line())
		}
	}
}

func TestUpdate_AProbeFailureIsExplainedByTheVerdictNeverInferredAsUnsubscribed(t *testing.T) {
	walled := errors.New("doctor live agy-tmux rc=81 pattern=\"\"")
	for verdict, want := range map[usageprobe.Verdict]string{
		usageprobe.VerdictHealthy:     "quota ruled out",
		usageprobe.VerdictUnavailable: "auth, install or network",
		usageprobe.VerdictUnknown:     "quota could not be verified",
	} {
		res, _ := explainedUpdate(t, walled, verdict)

		if res.Status != cliupdate.StatusSkipped || !strings.Contains(res.Detail, want) || !strings.Contains(res.Detail, "rc=81") || strings.Contains(res.Detail, "counts as unsubscribed") {
			t.Errorf("%s: result %+v; want skipped, explained by %q, never inferred as unsubscribed", verdict, res, want)
		}
	}
}

var smokeRefused = errors.New(`doctor live agy-tmux rc=85 pattern="exhausted"`)

var smokeOrigins = map[string]func() (*fakeHost, []cliupdate.Record){
	"a version found between boundaries": func() (*fakeHost, []cliupdate.Record) {
		return &fakeHost{versions: map[string][]string{"agy": {"1.4.1"}}, smokeErr: map[string]error{"agy": smokeRefused}}, agyLastSeenAt("1.4.0")
	},
	"a version the updater raised": func() (*fakeHost, []cliupdate.Record) {
		return &fakeHost{versions: map[string][]string{"agy": {"1.4.1", "1.4.2"}}, smokeErr: map[string]error{"agy": smokeRefused}}, nil
	},
	"a version that could not be read": func() (*fakeHost, []cliupdate.Record) {
		return &fakeHost{versionErr: map[string][]error{"agy": {errors.New("agy --version: killed")}}, smokeErr: map[string]error{"agy": smokeRefused}}, nil
	},
}

func geminiWeek(usedPct float64, exhausted bool) quotastate.UsageWindow {
	return quotastate.UsageWindow{Scope: "GEMINI MODELS", Kind: quotastate.KindWeek, PercentUsed: usedPct, ResetsText: "96h 0m", Family: "agy", Exhausted: exhausted}
}

func argonWeekDrained() quotastate.UsageWindow {
	return quotastate.UsageWindow{Scope: "Gemini 4 Argon", Kind: quotastate.KindWeek, PercentUsed: 100, ResetsText: "133h 0m", Family: "agy", Model: "Gemini 4 Argon", Exhausted: true}
}

func argonDrainedFamilyHealthy() usageprobe.Evidence {
	return usageprobe.Evidence{
		CLI: "agy", Family: "agy", Verdict: usageprobe.VerdictHealthy,
		Windows: []quotastate.UsageWindow{geminiWeek(43, false), argonWeekDrained()},
		Detail:  "GEMINI MODELS week window 43% used, resets 96h 0m; per-model window exhausted, which is no family cause but fails a phase on that model: Gemini 4 Argon week window 100% used, resets 133h 0m",
	}
}

func geminiFamilyDrained() usageprobe.Evidence {
	return usageprobe.Evidence{
		CLI: "agy", Family: "agy", Verdict: usageprobe.VerdictExhausted,
		Windows: []quotastate.UsageWindow{geminiWeek(100, true)},
		Detail:  "GEMINI MODELS week window 100% used, resets 96h 0m",
	}
}

func smokeFailedUpdate(origin string, ev *usageprobe.Evidence) (cliupdate.Report, []string) {
	h, history := smokeOrigins[origin]()
	var asked []string
	seams := h.seams()
	if ev != nil {
		seams.Explain = func(_ context.Context, family string) usageprobe.Evidence {
			asked = append(asked, family)
			return *ev
		}
	}
	return cliupdate.Update(context.Background(), []cliupdate.Family{agy()}, seams, history), asked
}

func TestUpdate_ASmokeFailureWhileOnlyAModelWindowIsExhaustedStillHaltsAndNamesTheWindow(t *testing.T) {
	ev := argonDrainedFamilyHealthy()
	for origin := range smokeOrigins {
		t.Run(origin, func(t *testing.T) {
			rep, asked := smokeFailedUpdate(origin, &ev)

			res := only(t, rep)
			if res.Status != cliupdate.StatusSmokeFailed || len(rep.SmokeFailed()) != 1 {
				t.Fatalf("the smoke ran the dispatched model, so a drained window of another model cannot explain its failure and the next wave must halt: %+v", res)
			}
			for _, want := range []string{"rc=85", "quota ruled out", "Gemini 4 Argon week window 100% used"} {
				if !strings.Contains(res.Detail, want) {
					t.Errorf("detail %q must carry %q", res.Detail, want)
				}
			}
			if strings.Contains(res.Detail, "verified the cause") {
				t.Errorf("a window of one model is a note, never the verified cause: %q", res.Detail)
			}
			if !reflect.DeepEqual(asked, []string{"agy"}) {
				t.Errorf("asked %v; want one usage query", asked)
			}
		})
	}
}

func TestUpdate_ASmokeFailureWhileTheFamilyIsExhaustedIsAVerifiedQuotaCauseNotAHalt(t *testing.T) {
	evidence := map[string]usageprobe.Evidence{
		"a drained family window": geminiFamilyDrained(),
		"an exhausted_regex wall": {CLI: "agy", Family: "agy", Verdict: usageprobe.VerdictExhausted, Detail: "the agy usage screen matched its manifest's exhausted_regex for agy; no window was read"},
	}
	for name, ev := range evidence {
		for origin := range smokeOrigins {
			t.Run(name+"/"+origin, func(t *testing.T) {
				rep, _ := smokeFailedUpdate(origin, &ev)

				res := only(t, rep)
				if res.Status != cliupdate.StatusQuotaExhausted || len(rep.SmokeFailed()) != 0 {
					t.Fatalf("an exhausted family explains the failed smoke: %+v", res)
				}
				if !strings.Contains(res.Detail, "quota exhausted, verified by a usage query") || !strings.Contains(res.Detail, "rc=85") {
					t.Errorf("detail %q must carry the verified cause and the smoke error", res.Detail)
				}
			})
		}
	}
}

func TestUpdate_ASmokeFailureWithHealthyUsageStillHaltsAndSaysQuotaIsRuledOut(t *testing.T) {
	ev := usageprobe.Evidence{CLI: "agy", Family: "agy", Verdict: usageprobe.VerdictHealthy, Windows: []quotastate.UsageWindow{geminiWeek(43, false)}, Detail: "GEMINI MODELS week window 43% used, resets 96h 0m"}
	for origin := range smokeOrigins {
		t.Run(origin, func(t *testing.T) {
			rep, asked := smokeFailedUpdate(origin, &ev)

			res := only(t, rep)
			if res.Status != cliupdate.StatusSmokeFailed || len(rep.SmokeFailed()) != 1 {
				t.Fatalf("no window is exhausted, so a failed smoke is a broken CLI and halts the next wave: %+v", res)
			}
			if !strings.Contains(res.Detail, "rc=85") || !strings.Contains(res.Detail, "quota ruled out") || len(asked) != 1 {
				t.Errorf("detail %q after %d usage queries; want the smoke error and the ruled-out quota", res.Detail, len(asked))
			}
		})
	}
}

func TestUpdate_ASmokeFailureWithoutAUsableUsageQueryStillHalts(t *testing.T) {
	evidence := map[string]*usageprobe.Evidence{
		"no usage seam":              nil,
		"the usage query failed":     {CLI: "agy", Family: "agy", Verdict: usageprobe.VerdictUnavailable, Detail: "usage workspace: no space left"},
		"quota could not be checked": {CLI: "agy", Family: "agy", Verdict: usageprobe.VerdictUnknown, Detail: "the agy usage screen showed 0 window(s), none for agy"},
	}
	for name, ev := range evidence {
		t.Run(name, func(t *testing.T) {
			rep, _ := smokeFailedUpdate("a version found between boundaries", ev)

			if res := only(t, rep); res.Status != cliupdate.StatusSmokeFailed || len(rep.SmokeFailed()) != 1 || !strings.Contains(res.Detail, "rc=85") {
				t.Fatalf("a usage query that proves nothing must not turn a failed smoke into a warning: %+v", res)
			}
		})
	}
}

func TestUpdate_AnInterruptedSmokeAsksNoUsageQueryAndIsNeverAQuotaCause(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"agy": {"1.4.1"}}, smokeErr: map[string]error{"agy": errors.New("signal: killed")}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.onSmoke = cancel
	var asked []string
	seams := h.seams()
	seams.Explain = func(_ context.Context, family string) usageprobe.Evidence {
		asked = append(asked, family)
		return geminiFamilyDrained()
	}

	res := only(t, cliupdate.Update(ctx, []cliupdate.Family{agy()}, seams, agyLastSeenAt("1.4.0")))

	if res.Status != cliupdate.StatusSkipped || !strings.HasPrefix(res.Detail, "interrupted") || len(asked) != 0 {
		t.Fatalf("an interrupt is skipped and explained by no usage query: %+v asked=%v", res, asked)
	}
}

func TestUpdate_AnInterruptDuringTheUsageQueryIsSkippedNeverSmokeFailedNorQuota(t *testing.T) {
	healthy := usageprobe.Evidence{CLI: "agy", Family: "agy", Verdict: usageprobe.VerdictHealthy, Windows: []quotastate.UsageWindow{geminiWeek(43, false)}, Detail: "GEMINI MODELS week window 43% used, resets 96h 0m"}
	evidence := map[string]usageprobe.Evidence{
		"an exhausted family": geminiFamilyDrained(),
		"healthy usage":       healthy,
	}
	for name, ev := range evidence {
		t.Run(name, func(t *testing.T) {
			h := &fakeHost{versions: map[string][]string{"agy": {"1.4.1"}}, smokeErr: map[string]error{"agy": smokeRefused}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			seams := h.seams()
			seams.Explain = func(context.Context, string) usageprobe.Evidence {
				cancel()
				return ev
			}

			rep := cliupdate.Update(ctx, []cliupdate.Family{agy()}, seams, agyLastSeenAt("1.4.0"))

			res := only(t, rep)
			if res.Status != cliupdate.StatusSkipped || !strings.HasPrefix(res.Detail, "interrupted") || len(rep.SmokeFailed()) != 0 || len(rep.Failed()) != 0 {
				t.Fatalf("SIGINT during the usage query is an interrupt, never a halt and never a quota cause: %+v", res)
			}
		})
	}
}
