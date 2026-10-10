//go:build integration

package bridge

import (
	"bytes"
	"context"
	"testing"
	"time"
)

type FakeREPLDispatch struct {
	Code      int
	Stderr    string
	WaitTicks int
	LossTick  int
}

func DispatchFakeREPLAndLoseServer(t *testing.T, ctx context.Context, perTick time.Duration, lose func()) FakeREPLDispatch {
	t.Helper()
	const marker = "PL-READY"
	cfg := itConfig(t, "ARTIFACT=/dev/null")
	cfg.ArtifactTimeoutS = 1200
	launchCmd := writeFakeREPL(t, cfg.Worktree, "artifact-timeout", marker)
	run := FakeREPLDispatch{LossTick: 2}
	var stderr bytes.Buffer
	deps := itDeps(perTick)
	deps.Stderr = &stderr
	deps.Sleep = func(d time.Duration) {
		if d == artifactWaitInterval {
			run.WaitTicks++
			if run.WaitTicks == run.LossTick {
				lose()
			}
		}
		time.Sleep(perTick)
	}
	run.Code, _ = runTmuxREPL(ctx, cfg, deps, itLaunch(itSession("panelost"), launchCmd, marker, 0, false))
	run.Stderr = stderr.String()
	return run
}
