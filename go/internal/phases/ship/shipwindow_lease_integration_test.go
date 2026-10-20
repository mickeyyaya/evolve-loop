//go:build integration

package ship

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestShip_TheShipWindowCoversTheBindingTheCarryGateAndThePush(t *testing.T) {
	t.Parallel()
	repo, _ := shipWorktreeFixture(t, "cycle-1-window")
	var mu sync.Mutex
	seen := map[string][]bool{}
	observe := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" {
			for _, verb := range []string{"push", "write-tree", "rev-parse HEAD"} {
				if strings.Contains(strings.Join(args, " "), verb) {
					mu.Lock()
					seen[verb] = append(seen[verb], leaseHeld(repo))
					mu.Unlock()
				}
			}
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1: goal=test", Runner: observe})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("ship failed: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	for _, verb := range []string{"push", "write-tree", "rev-parse HEAD"} {
		held := seen[verb]
		if len(held) == 0 {
			t.Fatalf("ship never ran git %s", verb)
		}
		for i, h := range held {
			if !h {
				t.Errorf("git %s call %d ran outside the ship window", verb, i)
			}
		}
	}
	if leaseHeld(repo) {
		t.Fatal("ship returned with the ship window still held")
	}
	if containsLog(res, "ship-window lease not acquired") {
		t.Fatalf("an uncontended ship logged an unleased ship: %v", res.Logs)
	}
}
