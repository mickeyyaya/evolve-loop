//go:build integration

package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A main update between screening and seeding must not dispatch any phase
// against the rejected snapshot, and neither old nor newly seeded work is lost.
func TestRunCycle_ContinuationAdvanceFailureStopsBeforeBuild(t *testing.T) {
	for _, failSeed := range []bool{false, true} {
		name := "base-advance"
		if failSeed {
			name = "seeding"
		}
		t.Run(name, func(t *testing.T) {
			root, wt := initContinuationRepo(t, 86)
			m := stampedContinuation(t, root, wt, 86)
			seedStampedInboxItem(t, root, 86, "task-a")
			runners := buildRunners(nil)
			runners[PhaseTriage] = &claimingTriageRunner{fakeRunner: runners[PhaseTriage].(*fakeRunner), root: root, taskID: "task-a"}
			provisioner := &racingContinuationProvisioner{t: t, failSeed: failSeed}
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithContinuationResolver(productionResolver(t)), WithWorktreeProvisioner(provisioner))
			result, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("expected explicit adoption failure, result=%+v err=%v", result, err)
			}
			for _, phase := range []Phase{PhaseTDD, PhaseBuild, PhaseAudit, PhaseShip} {
				if runners[phase].(*fakeRunner).calls != 0 {
					t.Errorf("%s dispatched after rejected adoption", phase)
				}
			}
			if got := gitOut(t, wt, "rev-parse", "HEAD"); got != m.SnapshotSHA {
				t.Error("original snapshot changed")
			}
			if _, err := os.Stat(filepath.Join(provisioner.seeded, "prior_work.go")); err != nil {
				t.Errorf("seeded work lost: %v", err)
			}
			if got := gitOut(t, provisioner.seeded, "status", "--porcelain"); strings.Contains(got, "UU ") {
				t.Errorf("merge conflict left unresolved: %s", got)
			}
		})
	}
}

type racingContinuationProvisioner struct {
	gitWorktree
	t        *testing.T
	seeded   string
	failSeed bool
}

func (p *racingContinuationProvisioner) CreateFrom(root string, cycle int, ref string) (string, error) {
	wt, err := p.gitWorktree.CreateFrom(root, cycle, ref)
	if err != nil {
		return wt, err
	}
	p.seeded = wt
	if p.failSeed {
		return wt, errors.New("injected seeding failure after replacement")
	}
	if err := os.WriteFile(filepath.Join(root, "prior_work.go"), []byte("package conflicting\n"), 0o644); err != nil {
		p.t.Fatal(err)
	}
	gitOut(p.t, root, "add", "prior_work.go")
	gitOut(p.t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "raced main update")
	return wt, nil
}
