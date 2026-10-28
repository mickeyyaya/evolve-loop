//go:build integration

package shiptest_test

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestShip_TheShipWindowCoversTheBindingTheCarryGateAndThePush(t *testing.T) {
	t.Parallel()
	repo, _ := shiptest.ShipWorktreeFixture(t, "cycle-1-window")
	var mu sync.Mutex
	seen := map[string][]bool{}
	observe := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" {
			for _, verb := range []string{"push", "write-tree", "rev-parse HEAD"} {
				if strings.Contains(strings.Join(args, " "), verb) {
					mu.Lock()
					seen[verb] = append(seen[verb], shiptest.LeaseHeld(repo))
					mu.Unlock()
				}
			}
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1: goal=test", Runner: observe})
	if err != nil || res.ExitCode != ship.ExitOK {
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
	if shiptest.LeaseHeld(repo) {
		t.Fatal("ship returned with the ship window still held")
	}
	if containsLog(res, "ship-window lease not acquired") {
		t.Fatalf("an uncontended ship logged an unleased ship: %v", res.Logs)
	}
}

func TestShip_AResumedLandingPushesInsideTheShipWindow(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	_, _, _, err := l.failEveryPush(t, shiptest.GithubServerError)
	shiptest.WantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassTransient, "")
	var pushes []bool
	observe := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && len(args) > 0 && args[0] == "push" {
			pushes = append(pushes, shiptest.LeaseHeld(l.Repo))
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}

	res, err := runShip(t, l.Repo, l.options(observe, &shiptest.SleepLog{}))

	if err != nil || res.RepairOutcome != "landing-resumed" {
		t.Fatalf("precondition: the retry resumes the prepared landing, got outcome=%q err=%v logs=%v", res.RepairOutcome, err, res.Logs)
	}
	if len(pushes) == 0 {
		t.Fatal("the resumed landing never pushed")
	}
	for i, held := range pushes {
		if !held {
			t.Errorf("resumed push %d ran outside the ship window", i)
		}
	}
	if shiptest.LeaseHeld(l.Repo) {
		t.Fatal("the resumed landing returned with the ship window still held")
	}
}
