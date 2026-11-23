package swarm

import (
	"context"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestReapOrphanSockets_ReapsSuffixedTestSocketsOfADeadOwner(t *testing.T) {
	suffixes := []string{"-hermetic", "-secret", "-failstart", "-twofp", "-earlykill", "-notmine"}
	var listed, want []string
	for _, s := range suffixes {
		listed = append(listed, bridge.DeriveTestSocket(111)+s, bridge.DeriveTestSocket(222)+s)
		want = append(want, bridge.DeriveTestSocket(111)+s)
	}
	list := func() ([]string, error) { return listed, nil }
	alive := func(pid int) bool { return pid == 222 }
	var killed []string
	kill := func(_ context.Context, socket string) error {
		killed = append(killed, socket)
		return nil
	}

	rep := ReapOrphanSockets(context.Background(), list, alive, kill)

	slices.Sort(killed)
	slices.Sort(want)
	if !slices.Equal(killed, want) || rep.SkippedLive != len(suffixes) {
		t.Fatalf("killed %v (skipped live %d); want every dead-owner suffixed socket %v reaped and live ones spared", killed, rep.SkippedLive, want)
	}
}
