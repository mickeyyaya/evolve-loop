package bridge

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

func TestItDeps_APaneThatOutlivesKillSessionIsReapedOnlyThroughTheInjectedSender(t *testing.T) {
	t.Parallel()
	const pane = 43610
	var events []string
	host := &hostTable{events: &events, tables: [][]proctree.Process{{hostProc(pane, 1, "")}}}
	var stderr bytes.Buffer
	deps := itDeps(0)
	deps.Tmux = &panePIDTmux{panePIDs: []int{pane}, events: &events}
	deps.ListProcesses = host.list
	deps.Stderr = &stderr

	if err := killSessionSwept(context.Background(), deps, "einject"); err != nil {
		t.Fatalf("killSessionSwept: %v", err)
	}

	if !strings.Contains(stderr.String(), "pid 43610") || !strings.Contains(stderr.String(), errRealTmuxTestSendsNoSignal.Error()) {
		t.Errorf("stderr = %q, want the injected sender to refuse pid %d", stderr.String(), pane)
	}
}
