package launchoutcome

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestClassify_ARouterUsageLimitExitNamesTheQuotaNotTheUnknownPrompt(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"the exhaustion escalation report": "[auto-respond] escalation report written (pattern=exhausted reason=escalate)\n",
		"the rate limit escalation report": "[auto-respond] escalation report written (pattern=rate_limit reason=escalate)\n",
		"the exhausted pane line":          "[claude-tmux] EXHAUSTED: pane shows a quota/rate-limit wall\n",
	}
	for name, stderr := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out := Classify(ExitUnknownPrompt, nil, stderr)

			if out.Signal == CodeExitUnknownPrompt || !strings.Contains(string(out.Signal), "QUOTA") {
				t.Errorf("signal = %q, want a code that names the quota and not %s", out.Signal, CodeExitUnknownPrompt)
			}
			if _, registered := signalcenter.IsRegistered(out.Signal); !registered {
				t.Errorf("signal %q is not registered with the signal center", out.Signal)
			}
			if !out.Transient {
				t.Errorf("a quota wall exit must stay transient so the retry and the pause path see it: %+v", out)
			}
		})
	}
}

func TestClassify_AnUnknownPromptExitKeepsTheUnknownPromptSignal(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"an empty pane":                 "",
		"an unnamed pattern":            "[auto-respond] escalation report written (pattern=unknown reason=escalate)\n",
		"a model wall that is no quota": "[auto-respond] escalation report written (pattern=model_unsupported reason=escalate)\n",
	}
	for name, stderr := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out := Classify(ExitUnknownPrompt, nil, stderr)

			if out.Signal != CodeExitUnknownPrompt {
				t.Errorf("signal = %q, want %s: only a quota wall earns the quota code", out.Signal, CodeExitUnknownPrompt)
			}
		})
	}
}
