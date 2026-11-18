package agentcmd

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestAgentRun_AVerdictLineInACodeFenceReadsAsTheVerdictThatItStates(t *testing.T) {
	cases := []struct {
		name, report, verdict string
		exit                  int
	}{
		{"a fenced PASS", threatModel + "## Findings\nNone.\n## Verdict\n```\nVerdict: PASS\n```\n", core.VerdictPASS, 0},
		{"a fenced FAIL", threatModel + "## Findings\nNone.\n## Verdict\n```\nVerdict: FAIL\n```\n", core.VerdictFAIL, exitFail},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, code, center, _ := runWithSignals(t, c.report)

			if env.Verdict != c.verdict || code != c.exit {
				t.Fatalf("verdict %q exit %d, want %s and exit %d; diagnostics %+v", env.Verdict, code, c.verdict, c.exit, env.Diagnostics)
			}
			if defects := parseDefectSignals(center); len(defects) != 0 {
				t.Errorf("parse defect signals = %+v, want none for a report that states a verdict in a fence", defects)
			}
		})
	}
}

func TestAgentRun_AReportWithNoVerdictInOrOutOfAFenceStillFails(t *testing.T) {
	env, code, _, _ := runWithSignals(t, threatModel+"## Findings\nNone.\n## Verdict\nsee above\n```\nno verdict here\n```\n")

	if env.Verdict != core.VerdictFAIL || code != exitFail {
		t.Fatalf("verdict %q exit %d, want FAIL and exit 1; diagnostics %+v", env.Verdict, code, env.Diagnostics)
	}
}

func TestAgentRun_AFencedVerdictOnlyReportEmitsOneParseDefect(t *testing.T) {
	t.Parallel()

	env, code, _, _ := runWithSignals(t, threatModel+"## Findings\nNone.\n## Verdict\n```\nno verdict here\n```\n")

	if env.Verdict != core.VerdictFAIL || code != exitFail {
		t.Fatalf("verdict %q exit %d, want FAIL and exit 1", env.Verdict, code)
	}
	bad := 0
	for _, d := range env.Diagnostics {
		if d.Severity == "error" && strings.Contains(d.Message, "bad_verdict") {
			bad++
		}
	}
	if bad != 1 {
		t.Errorf("bad_verdict errors = %d, want 1; %+v", bad, env.Diagnostics)
	}
}
