package cliupdate_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
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
