package opscmd

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

var failingStageLine = regexp.MustCompile(`failing stage: (launch|trust prompt|REPL prompt|delivery|artifact)\b`)

const trustDialogPane = "Do you trust the files in this folder?\n1. Yes, proceed\n2. No, exit\n"

func liveFailureStderr(rc int, scrollback string) (string, int) {
	var out, errb bytes.Buffer
	code := reportDoctorLiveResult(liveOutcome{target: liveProbeTarget{driver: "claude-tmux"}, rc: rc, scrollback: scrollback}, &out, &errb)
	return errb.String(), code
}

func TestDoctorLive_FailureNamesItsFailingStage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		rc         int
		scrollback string
		wantStage  string
	}{
		{"missing binary fails at launch", bridge.ExitMissingBinary, "", "launch"},
		{"safety gate fails at launch", bridge.ExitSafetyGate, "", "launch"},
		{"no REPL marker fails at the REPL prompt", bridge.ExitREPLBootTimeout, paneLines(1, 20) + "\n", "REPL prompt"},
		{"a pending folder-trust dialog fails at the trust prompt", bridge.ExitRespondLoopGuard, trustDialogPane, "trust prompt"},
		{"no artifact before the deadline fails at the artifact", bridge.ExitArtifactTimeout, paneLines(1, 20) + "\n", "artifact"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stderr, code := liveFailureStderr(c.rc, c.scrollback)
			if code != exitFail {
				t.Errorf("code = %d, want exitFail", code)
			}
			got := failingStageLine.FindAllStringSubmatch(stderr, -1)
			if len(got) != 1 || got[0][1] != c.wantStage {
				t.Fatalf("stderr names stages %v, want exactly %q:\n%s", got, c.wantStage, stderr)
			}
		})
	}
}

func TestDoctorLive_EveryFailureCodeNamesExactlyOneStageInsideTheWaveTail(t *testing.T) {
	t.Parallel()
	for _, rc := range []int{bridge.ExitSafetyGate, bridge.ExitCostLeak, bridge.ExitREPLBootTimeout, bridge.ExitArtifactTimeout, bridge.ExitRespondLoopGuard, bridge.ExitRequireFullUnmet, bridge.ExitCmdTimeout, bridge.ExitMissingBinary, 1} {
		stderr, _ := liveFailureStderr(rc, paneLines(1, 60)+"\n")
		if n := len(failingStageLine.FindAllString(stderr, -1)); n != 1 {
			t.Errorf("rc=%d names %d failing stages, want exactly 1:\n%s", rc, n, stderr)
			continue
		}
		lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
		at := -1
		for i, l := range lines {
			if failingStageLine.MatchString(l) {
				at = i
			}
		}
		if from := len(lines) - at; from > 40 {
			t.Errorf("rc=%d stage line is %d lines from the end, outside a 40-line tail:\n%s", rc, from, stderr)
		}
	}
}

func TestDoctorLive_HealthyProbeNamesNoFailingStage(t *testing.T) {
	t.Parallel()
	stderr, code := liveFailureStderr(bridge.ExitOK, paneLines(1, 20)+"\n")
	if code != exitOK || strings.Contains(stderr, "failing stage") {
		t.Errorf("healthy probe: code %d, stderr %q; want exitOK and no failing stage", code, stderr)
	}
}
