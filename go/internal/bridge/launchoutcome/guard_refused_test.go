package launchoutcome

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func TestTimeoutGuardRefused_IsTheCauseThatEndsTheChain(t *testing.T) {
	t.Parallel()
	if string(TimeoutGuardRefused) != llmroute.CauseGuardRefused {
		t.Fatalf("the bridge cause %q and the chain cause %q must be one token", TimeoutGuardRefused, llmroute.CauseGuardRefused)
	}
}

func TestClassify_AStallAfterAGuardDenialNamesTheGuardAndThePath(t *testing.T) {
	t.Parallel()
	stderr := "[bridge] FAIL: completion never signalled\n" +
		`[bridge] artifact-timeout: cause=guard_refused reason="guard role denied /repo/go/internal/x.go: role guard: phase=build may not write outside workspace" phase=build cycle=7 driver=claude-tmux` + "\n"

	out := Classify(ExitArtifactTimeout, nil, stderr)

	if out.CauseCode != string(TimeoutGuardRefused) {
		t.Fatalf("cause %q, want %q", out.CauseCode, TimeoutGuardRefused)
	}
	msg := out.Err.Error()
	if !strings.Contains(msg, "guard role") || !strings.Contains(msg, "/repo/go/internal/x.go") || strings.Contains(msg, "quota") {
		t.Fatalf("the summary %q must name the guard and the path and never say quota", msg)
	}
}
