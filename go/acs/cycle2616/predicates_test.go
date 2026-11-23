//go:build acs

// Package cycle2616 encodes the acceptance of the test-tmux-servers-die-with-their-test-process inbox item: the tmux server of a dead test binary does not outlive the next test binary start or the next gc run.
package cycle2616

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var ownerSuffixes = []string{"", "-own", "-hermetic", "-secret", "-failstart", "-twofp", "-earlykill", "-notmine"}

func serverUp(socket string) bool {
	return exec.Command("tmux", "-L", socket, "display-message", "-p", "#{pid}").Run() == nil
}

func plantServer(t *testing.T, socket string) {
	t.Helper()
	out, err := exec.Command("tmux", "-L", socket, "-f", os.DevNull, "start-server", ";", "set-option", "-s", "exit-empty", "off").CombinedOutput()
	if err != nil {
		t.Fatalf("start a tmux server on %s: %v: %s", socket, err, out)
	}
	t.Cleanup(func() { _ = exec.Command("tmux", "-L", socket, "kill-server").Run() })
	if !serverUp(socket) {
		t.Fatalf("the planted server on %s is not up", socket)
	}
}

func liveOwnerPID(t *testing.T) int {
	t.Helper()
	owner := exec.Command("sleep", "120")
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Process.Kill(); _, _ = owner.Process.Wait() })
	return owner.Process.Pid
}

type startProbe struct {
	watched map[string]bool
	seen    map[string]bool
}

func (p *startProbe) Run() int {
	for range 50 {
		stopped := true
		for socket, shouldStop := range p.watched {
			if shouldStop && serverUp(socket) {
				stopped = false
			}
		}
		if stopped {
			break
		}
		exec.Command("sleep", "0.1").Run()
	}
	for socket := range p.watched {
		p.seen[socket] = serverUp(socket)
	}
	return 0
}

func TestC2616_001_TheNextTestBinaryStartStopsTheServersOfDeadTestBinaries(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatalf("tmux is required to prove a real server is stopped: %v", err)
	}
	dir, err := os.MkdirTemp("/tmp", "c26")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMUX_TMPDIR", dir)
	t.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(1))

	deadBare, deadSuffixed := acsassert.ExitedPID(t), acsassert.ExitedPID(t)
	live := liveOwnerPID(t)
	probe := &startProbe{watched: map[string]bool{}, seen: map[string]bool{}}
	plant := func(socket string, shouldStop bool) {
		plantServer(t, socket)
		probe.watched[socket] = shouldStop
	}
	plant(bridge.DeriveTestSocket(deadBare), true)
	plant(bridge.DeriveTestSocket(deadSuffixed)+"-hermetic", true)
	plant(bridge.DeriveTestSocket(deadSuffixed)+"-earlykill", true)
	plant(bridge.DeriveTestSocket(live), false)
	plant(bridge.DeriveTestSocket(live)+"-hermetic", false)

	if code := tmuxtest.Main(probe); code != 0 {
		t.Fatalf("tmuxtest.Main exit=%d", code)
	}

	for socket, shouldStop := range probe.watched {
		switch {
		case shouldStop && probe.seen[socket]:
			t.Errorf("%s, the server of a dead test binary, is still up after the next test binary started", socket)
		case !shouldStop && !probe.seen[socket]:
			t.Errorf("%s, the server of a live test binary, was stopped", socket)
		}
	}
}

func TestC2616_002_AGcRunReapsEverySuffixedServerOfADeadOwnerAndSparesLiveOnes(t *testing.T) {
	const deadPID, livePID = 111, 222
	var listed, want, spared []string
	for _, suffix := range ownerSuffixes {
		listed = append(listed, bridge.DeriveTestSocket(deadPID)+suffix, bridge.DeriveTestSocket(livePID)+suffix)
		want = append(want, bridge.DeriveTestSocket(deadPID)+suffix)
		spared = append(spared, bridge.DeriveTestSocket(livePID)+suffix)
	}
	listed = append(listed, bridge.DeriveRunSocket(deadPID))
	want = append(want, bridge.DeriveRunSocket(deadPID))
	notOwned := []string{"evolve-bridge-t", "evolve-bridge-tabc", bridge.DeriveTestSocket(deadPID) + "x", bridge.DeriveTestSocket(0) + "-hermetic", "other-bridge-t111-hermetic", bridge.TmuxSocket}
	listed = append(listed, notOwned...)
	var killed []string
	kill := func(_ context.Context, socket string) error {
		killed = append(killed, socket)
		return nil
	}

	rep := swarm.ReapOrphanSockets(context.Background(), func() ([]string, error) { return listed, nil }, func(pid int) bool { return pid == livePID }, kill)

	slices.Sort(killed)
	slices.Sort(want)
	if !slices.Equal(killed, want) {
		t.Errorf("killed %v, want exactly the dead-owner sockets %v", killed, want)
	}
	if rep.SkippedLive != len(spared) {
		t.Errorf("skipped live = %d, want %d (%v)", rep.SkippedLive, len(spared), spared)
	}
	for _, socket := range killed {
		if slices.Contains(spared, socket) || slices.Contains(notOwned, socket) {
			t.Errorf("killed %s, which belongs to a live owner or to no owner", socket)
		}
	}
}

func TestC2616_003_GcDryRunListsDeadOwnerTestServersAndApplyStopsThemNeverALiveOne(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(TestGCSockets_ADryRunNamesEveryDeadOwnerTestServerAndNeverALiveOne|TestGCSockets_AnApplyStopsEveryDeadOwnerTestServerAndSparesALiveOne)$",
		Names: []string{
			"TestGCSockets_ADryRunNamesEveryDeadOwnerTestServerAndNeverALiveOne",
			"TestGCSockets_AnApplyStopsEveryDeadOwnerTestServerAndSparesALiveOne",
		},
	})
}

func TestC2616_004_BothRulesArePinnedByAFakeProcessTableThatKillsNoHostProcess(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/swarm",
		Pattern: "^(TestReapOrphanSockets_ReapsSuffixedTestSocketsOfADeadOwner)$",
		Names:   []string{"TestReapOrphanSockets_ReapsSuffixedTestSocketsOfADeadOwner"},
	})
}
