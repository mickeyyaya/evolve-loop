package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const closeoutCommit = "dossier: cycle-1705 closeout\n\nknowledge-base/cycles/cycle-1705.json\nknowledge-base/cycles/cycle-1705.md"

func writePendingCloseout(root string) error {
	d := &dossier.Dossier{Cycle: 1705, Goal: "a lane's closeout", FinalVerdict: dossier.VerdictPass,
		Phases:         []dossier.PhaseRecord{{Name: "build", Verdict: dossier.VerdictPass}},
		SpineFailOpens: []cyclestate.SpineFailOpen{{Phase: "audit", MissingArtifact: "build-report.md"}}}
	return dossier.Write(d, dossier.PendingDir(root), false)
}

func gitPlane(t *testing.T, branch string) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("symbolic-ref", "HEAD", "refs/heads/"+branch)
	if err := os.WriteFile(filepath.Join(r.Dir, "README.md"), []byte("plane\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", "README.md")
	r.Git("commit", "-q", "-m", "base")
	return r
}

func planeWithAPendingCloseout(t *testing.T, branch string) *gittest.Repo {
	t.Helper()
	r := gitPlane(t, branch)
	if err := writePendingCloseout(r.Dir); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPublishPendingDossiers_PublishesTheLanesPendingCloseouts(t *testing.T) {
	for _, branch := range []string{"main", "fleet-run"} {
		t.Run(branch, func(t *testing.T) {
			r := planeWithAPendingCloseout(t, branch)
			var warn bytes.Buffer

			publishPendingDossiers(r.Dir, &warn)

			if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); got != closeoutCommit {
				t.Fatalf("HEAD = %q, want one commit of cycle 1705's closeout", got)
			}
			if !strings.Contains(warn.String(), "published cycles [1705]") {
				t.Fatalf("the publish is not reported: %q", warn.String())
			}
		})
	}
}

func TestPublishPendingDossiers_HoldsThePairsUnlessThePlaneHasOriginsHistory(t *testing.T) {
	for _, tc := range []struct {
		name      string
		setup     []string
		published bool
		warning   string
	}{
		{"current", []string{"update-ref refs/remotes/origin/main HEAD"}, true, ""},
		{"ahead", []string{"update-ref refs/remotes/origin/main HEAD", "commit -q --allow-empty -m local"}, true, ""},
		{"behind", []string{"commit -q --allow-empty -m upstream", "update-ref refs/remotes/origin/main HEAD", "reset -q --hard HEAD~1"}, false, "BEHIND origin/main"},
		{"diverged", []string{"commit -q --allow-empty -m upstream", "update-ref refs/remotes/origin/main HEAD", "reset -q --hard HEAD~1", "commit -q --allow-empty -m local"}, false, "DIVERGED"},
		{"origin never fetched", nil, false, "rev-parse origin/main"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := planeWithAPendingCloseout(t, "main")
			r.Git("remote", "add", "origin", filepath.Join(r.Dir, "unreachable.git"))
			for _, step := range tc.setup {
				r.Git(strings.Fields(step)...)
			}
			head := r.Git("rev-parse", "HEAD")
			var warn bytes.Buffer

			publishPendingDossiers(r.Dir, &warn)

			if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); (got == closeoutCommit) != tc.published {
				t.Fatalf("published = %v, want %v (HEAD %q)", got == closeoutCommit, tc.published, got)
			}
			if tc.published {
				return
			}
			if r.Git("rev-parse", "HEAD") != head {
				t.Fatal("a held publish moved HEAD")
			}
			if _, err := os.Stat(filepath.Join(dossier.PendingDir(r.Dir), "cycle-1705.json")); err != nil {
				t.Fatalf("a held pair must stay pending: %v", err)
			}
			if !strings.Contains(warn.String(), tc.warning) || !strings.Contains(warn.String(), "they stay pending") {
				t.Fatalf("the hold must name the relation and keep the pairs: %q", warn.String())
			}
		})
	}
}

func leaseARun(t *testing.T, root string, cycle, ownerPID int) {
	t.Helper()
	runDir := cycleWorkspace(root, cycle)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "run-" + strconv.Itoa(cycle), OwnerPID: ownerPID}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestPublishPendingDossiers_RefusesWhileAnotherRunIsLive(t *testing.T) {
	for _, tc := range []struct {
		name      string
		owner     func(t *testing.T) int
		published bool
	}{
		{"a live run another process owns", func(*testing.T) int { return os.Getppid() }, false},
		{"the caller's own run", func(*testing.T) int { return os.Getpid() }, true},
		{"a sealed lane whose process exited", iscsExitedPID, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := planeWithAPendingCloseout(t, "main")
			leaseARun(t, r.Dir, 1706, tc.owner(t))
			head := r.Git("rev-parse", "HEAD")
			var warn bytes.Buffer

			publishPendingDossiers(r.Dir, &warn)

			if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); (got == closeoutCommit) != tc.published {
				t.Fatalf("published = %v, want %v; warn=%q", got == closeoutCommit, tc.published, warn.String())
			}
			if tc.published {
				return
			}
			if r.Git("rev-parse", "HEAD") != head {
				t.Fatal("a held publish moved HEAD")
			}
			if _, err := os.Stat(filepath.Join(dossier.PendingDir(r.Dir), "cycle-1705.json")); err != nil {
				t.Fatalf("a held pair must stay pending: %v", err)
			}
			if !strings.Contains(warn.String(), "another run is live") || !strings.Contains(warn.String(), "they stay pending") {
				t.Fatalf("the hold must say why and keep the pairs: %q", warn.String())
			}
		})
	}
}

func TestLoopSummary_ASecondLoopExitingBesideALiveRunPublishesNothing(t *testing.T) {
	r := planeWithAPendingCloseout(t, "main")
	leaseARun(t, r.Dir, 1706, os.Getppid())
	head := r.Git("rev-parse", "HEAD")
	lr := &loopResult{StopReason: "owned_by_live_run", classifyRoot: r.Dir}

	lr.emit(io.Discard)

	if r.Git("rev-parse", "HEAD") != head {
		t.Fatal("a loop that found another run live committed a pending closeout under it")
	}
	if _, err := os.Stat(filepath.Join(dossier.PendingDir(r.Dir), "cycle-1705.json")); err != nil {
		t.Fatalf("the pair must stay pending for the live run's own boundary: %v", err)
	}
}

func TestPublishPendingDossiers_OutsideACheckoutLeavesThePairsPendingSilently(t *testing.T) {
	root := t.TempDir()
	if err := writePendingCloseout(root); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer

	publishPendingDossiers(root, &warn)

	if warn.Len() != 0 {
		t.Fatalf("a root that is not a git checkout must stay silent, got %q", warn.String())
	}
	if _, err := os.Stat(filepath.Join(dossier.PendingDir(root), "cycle-1705.json")); err != nil {
		t.Fatalf("the pair must stay pending: %v", err)
	}
	if _, err := os.Stat(dossier.CyclesDir(root)); !os.IsNotExist(err) {
		t.Fatalf("nothing may reach the corpus outside a checkout: %v", err)
	}
}

func TestPublishPendingDossiers_WithNothingPendingNeverWaitsOnTheGitMutationLock(t *testing.T) {
	r := gitPlane(t, "main")
	release, err := flock.Lock(flock.ShipLockPath(r.Dir))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	done := make(chan struct{})

	go func() {
		publishPendingDossiers(r.Dir, io.Discard)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("publishPendingDossiers waited on the git-mutation lock with nothing pending")
	}
}

func TestLoopSummaryCountsTheLastWavesPendingCloseouts(t *testing.T) {
	r := planeWithAPendingCloseout(t, "main")
	lr := &loopResult{StopReason: "max_cycles", classifyRoot: r.Dir, batchFirstCycle: 1700}

	lr.emit(io.Discard)

	if lr.SpineFailOpens == nil || lr.SpineFailOpens.Total != 1 {
		t.Fatalf("the summary must count cycle 1705's spine fail-open, got %+v", lr.SpineFailOpens)
	}
	if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); got != closeoutCommit {
		t.Fatalf("HEAD = %q, want the last wave's closeout published at exit", got)
	}
}

func TestPrepareIteration_PublishesPendingCloseoutsOntoTheSyncedPlane(t *testing.T) {
	installCheckRunsGH(t, `{"total_count":1,"check_runs":[{"name":"build+test","status":"completed","conclusion":"success"}]}`, false)
	origin, seed := gittest.Bare(t), gittest.Fixture(t)
	seed.Git("commit", "--allow-empty", "-q", "-m", "base")
	seed.Git("push", "-q", origin.Dir, "main")
	plane := gittest.Clone(t, origin.Dir)
	seed.Git("commit", "--allow-empty", "-q", "-m", "landed upstream")
	seed.Git("push", "-q", origin.Dir, "main")
	if err := writePendingCloseout(plane.Dir); err != nil {
		t.Fatal(err)
	}
	leaseARun(t, plane.Dir, 1704, os.Getpid())
	leaseARun(t, plane.Dir, 1705, iscsExitedPID(t))
	evolveDir := filepath.Join(plane.Dir, ".evolve")
	var console bytes.Buffer
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: plane.Dir, EvolveDir: evolveDir},
		cycleEnv: map[string]string{"EVOLVE_CLI_HEALTH": "0"}, result: &loopResult{}, stdout: &console, stderr: &console}
	b.deps.Signals = newRootSignalCenter(plane.Dir, evolveDir, &console)
	fc, bin := policy.FleetConfig{Count: 1}, ""

	d := b.prepareIteration(0, &fc, &bin, 0)
	b.deps.Signals.Flush()

	if d.flow != batchProceed {
		t.Fatalf("the boundary must proceed, got %+v; console:\n%s", d, console.String())
	}
	if got := plane.Git("show", "--name-only", "--format=%s", "HEAD"); got != closeoutCommit {
		t.Fatalf("HEAD = %q, want the pending closeout published at the boundary; console:\n%s", got, console.String())
	}
	if plane.Git("rev-parse", "HEAD~1") != seed.Git("rev-parse", "HEAD") {
		t.Fatal("the closeout must land on main after the sync fast-forwarded it to origin/main")
	}
}

func TestCampaignRun_PublishesTheWavesPendingCloseouts(t *testing.T) {
	plane := gitPlane(t, "main")
	withCampaignLaunchFactory(t, func(_ string, _ bool, root, _, _ string, _, _ io.Writer) fleet.LaunchFn {
		return func(context.Context, fleet.CycleSpec) (int, error) {
			return 0, writePendingCloseout(root)
		}
	})
	var stdout, stderr bytes.Buffer

	code := runCampaignRun([]string{"--plan", writeCampaignTestPlan(t), "--project-root", plane.Dir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("runCampaignRun = %d, want 0; stderr=%s", code, stderr.String())
	}
	if got := plane.Git("show", "--name-only", "--format=%s", "HEAD"); got != closeoutCommit {
		t.Fatalf("HEAD = %q, want the wave's pending closeout published once its lanes returned", got)
	}
}

func TestRunFleet_RunsItsLanesThroughThePublishingRunner(t *testing.T) {
	n, err := acsassert.CountInGoFunc("cmd_fleet.go", "runFleet", "runLanesThenPublish(")
	if err != nil || n != 1 {
		t.Fatalf("runFleet must run its lanes through runLanesThenPublish once, got %d (%v)", n, err)
	}
	if n, _ := acsassert.CountInGoFunc("cmd_fleet.go", "runFleet", "sup.Run("); n != 0 {
		t.Fatalf("runFleet must not run its lanes around the publishing runner, found %d direct sup.Run call(s)", n)
	}
}
