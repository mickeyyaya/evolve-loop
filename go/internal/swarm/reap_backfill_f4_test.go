package swarm

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestReapOrphanSockets_SkipsUnparseableAndNonPositiveOwnersAndReportsKillFailures(t *testing.T) {
	list := func() ([]string, error) {
		return []string{
			"evolve-bridge-p0",
			"evolve-bridge-p99999999999999999999999",
			"evolve-bridge-p4242",
			"evolve-bridge-t4343",
			"evolve-bridge-p5151",
			"not-a-bridge-socket",
		}, nil
	}
	var attempted []string
	kill := func(_ context.Context, s string) error {
		attempted = append(attempted, s)
		if s == "evolve-bridge-p4242" {
			return errors.New("no server on socket")
		}
		return nil
	}

	rep := ReapOrphanSockets(context.Background(), list, aliveSet(5151), kill)

	if !reflect.DeepEqual(attempted, []string{"evolve-bridge-p4242", "evolve-bridge-t4343"}) {
		t.Errorf("only dead, parseable, positive owners are killed, in order: %v", attempted)
	}
	if !reflect.DeepEqual(rep.Killed, []string{"evolve-bridge-t4343"}) {
		t.Errorf("Killed = %v", rep.Killed)
	}
	if !reflect.DeepEqual(rep.Errors, []string{"evolve-bridge-p4242: no server on socket"}) {
		t.Errorf("Errors = %v", rep.Errors)
	}
	if rep.SkippedLive != 1 {
		t.Errorf("SkippedLive = %d", rep.SkippedLive)
	}
}
