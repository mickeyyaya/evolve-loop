package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func rebasedCarrierLane(t *testing.T) *shippedLane {
	t.Helper()
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	if declined, err := unwindShipCommit(context.Background(), fx.worktree, fx.audited(), gitCapture); err != nil || declined != "" {
		t.Fatalf("unwindShipCommit = (%q, %v)", declined, err)
	}
	fx.git("rebase", "-q", "main")
	return fx
}

func TestCarrierAtHead_RecognisesOnlyThisCycleAndRunsCarrier(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, label string
		lane        func(t *testing.T) *shippedLane
		want        bool
	}{
		{"this run's rebased carrier", "cycle-42/run-42", rebasedCarrierLane, true},
		{"a carrier of another run", "cycle-42/run-43", rebasedCarrierLane, false},
		{"a carrier of another cycle", "cycle-41/run-42", rebasedCarrierLane, false},
		{"a label that only starts with this one", "cycle-42/run-4", rebasedCarrierLane, false},
		{"a ship commit with no carrier trailer", "cycle-42/run-42", func(t *testing.T) *shippedLane {
			return newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := tc.lane(t)

			got, err := carrierAtHead(context.Background(), fx.worktree, tc.label, gitCapture)

			if err != nil || got != tc.want {
				t.Fatalf("carrierAtHead = (%v, %v), want %v", got, err, tc.want)
			}
		})
	}
}

func TestPendOnForkPoint_LeavesTheRebasedCarrierPendingOnTheNewBase(t *testing.T) {
	t.Parallel()
	fx := rebasedCarrierLane(t)

	onto, err := pendOnForkPoint(context.Background(), fx.worktree, gitCapture)

	if err != nil || onto != fx.newBase {
		t.Fatalf("pendOnForkPoint = (%q, %v), want %s", onto, err, fx.newBase)
	}
	fx.requirePendingLaneChangeOn(fx.newBase)
}

func TestCarrierHealGit_AFailureIsAnError(t *testing.T) {
	t.Parallel()
	for _, failing := range []string{"log", "merge-base", "reset", "rev-parse"} {
		t.Run(failing, func(t *testing.T) {
			t.Parallel()
			git := func(_ context.Context, _ string, args ...string) (string, int, error) {
				if args[0] == failing {
					return "", 0, errors.New(failing + " failed")
				}
				return "fork-sha\n", 0, nil
			}

			var err error
			switch failing {
			case "log":
				_, err = carrierAtHead(context.Background(), "/wt", "cycle-42/run-42", git)
			case "rev-parse":
				var rebasing bool
				rebasing, err = rebaseInProgress(context.Background(), "/wt", git)
				if rebasing {
					t.Error("rebaseInProgress = true after a failure, want false")
				}
			default:
				var onto string
				onto, err = pendOnForkPoint(context.Background(), "/wt", git)
				if onto != "" {
					t.Errorf("onto = %q after a failure, want empty", onto)
				}
			}

			if err == nil || !strings.Contains(err.Error(), failing+" failed") {
				t.Fatalf("err = %v, want the %s error", err, failing)
			}
		})
	}
}

func TestLoadResumeBootstrap_PendsARebasedCarrierBeforeAnyPhaseRuns(t *testing.T) {
	t.Parallel()
	fx := rebasedCarrierLane(t)
	center := signalcenter.New()
	events := recordSignals(center)
	var sink injectedStderr
	o := NewOrchestrator(&fakeStorage{cycleState: fx.cycleState()}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center), WithStderr(&sink))

	boot, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: fx.root}, &ResumePoint{CycleID: unwindCycle}, PhaseShip)

	if err != nil {
		t.Fatalf("loadResumeBootstrap: %v", err)
	}
	fx.requirePendingLaneChangeOn(fx.newBase)
	if boot.startPhase != PhaseShip {
		t.Errorf("start phase = %s, want ship kept: the pended change is the audited shape", boot.startPhase)
	}
	coded := codedEvents(*events, CodeResumeCarrierPended)
	if len(coded) != 1 || coded[0].Cycle != unwindCycle || coded[0].RunID != unwindRunID || !strings.Contains(coded[0].Reason, fx.newBase) {
		t.Fatalf("signals = %+v, want one %s naming cycle %d, %s and the fork point", coded, CodeResumeCarrierPended, unwindCycle, unwindRunID)
	}
	if !strings.Contains(sink.String(), "pended the carrier") {
		t.Errorf("stderr does not say the carrier was pended:\n%s", sink.String())
	}
}

func TestLoadResumeBootstrap_LeavesAnotherRunsCarrierAndAnInPlaceWorktreeAlone(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(t *testing.T, fx *shippedLane, cs CycleState) (CycleState, string)
	}{
		{"another run's carrier", func(t *testing.T, fx *shippedLane, cs CycleState) (CycleState, string) {
			cs.RunID = "run-43"
			return cs, t.TempDir()
		}},
		{"an in-place worktree", func(t *testing.T, fx *shippedLane, cs CycleState) (CycleState, string) {
			return cs, fx.worktree
		}},
		{"no run id yet", func(t *testing.T, fx *shippedLane, cs CycleState) (CycleState, string) {
			cs.RunID = ""
			return cs, t.TempDir()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := rebasedCarrierLane(t)
			head := fx.git("rev-parse", "HEAD")
			state := fx.cycleState()
			state.ExplanationDocumentationVersion = 0
			cs, root := tc.edit(t, fx, state)
			center := signalcenter.New()
			events := recordSignals(center)
			o := NewOrchestrator(&fakeStorage{cycleState: cs}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center))

			if _, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{CycleID: unwindCycle}, PhaseShip); err != nil {
				t.Fatalf("loadResumeBootstrap: %v", err)
			}

			if got := fx.git("rev-parse", "HEAD"); got != head {
				t.Fatalf("HEAD moved from %s to %s", head, got)
			}
			if coded := codedEvents(*events, CodeResumeCarrierPended); len(coded) != 0 {
				t.Fatalf("signals = %+v, want none", coded)
			}
		})
	}
}

func TestLoadResumeBootstrap_AWorktreeThatIsNotAGitCheckoutIsNotHealed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cs := CycleState{CycleID: 7, RunID: "run-7", WorkspacePath: filepath.Join(root, "ws"), ActiveWorktree: t.TempDir()}
	center := signalcenter.New()
	events := recordSignals(center)
	o := NewOrchestrator(&fakeStorage{cycleState: cs}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center))

	if _, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{CycleID: 7}, PhaseShip); err != nil {
		t.Fatalf("loadResumeBootstrap: %v", err)
	}

	if coded := codedEvents(*events, CodeResumeCarrierPended); len(coded) != 0 {
		t.Fatalf("signals = %+v, want none", coded)
	}
}

func TestHealCarrierAtHead_AGitFailureInAGitCheckoutFailsTheResumeAndMovesNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, failing, want string
		makeGit             func(worktree string) error
	}{
		{"a HEAD read in a checkout", "log", "read the HEAD of lane %s: log failed", func(wt string) error { return os.Mkdir(filepath.Join(wt, ".git"), 0o755) }},
		{"a rebase check in a checkout", "rev-parse", "check lane %s for a rebase in progress: rev-parse failed", func(wt string) error { return os.Mkdir(filepath.Join(wt, ".git"), 0o755) }},
		{"a HEAD read in a linked worktree", "log", "read the HEAD of lane %s: log failed", func(wt string) error {
			return os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /elsewhere/.git/worktrees/lane\n"), 0o644)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			worktree := t.TempDir()
			if err := tc.makeGit(worktree); err != nil {
				t.Fatal(err)
			}
			git := func(_ context.Context, _ string, args ...string) (string, int, error) {
				switch args[0] {
				case tc.failing:
					return "", 0, errors.New(tc.failing + " failed")
				case "log":
					return fmt.Sprintf("cycle-%d/%s\n", unwindCycle, unwindRunID), 0, nil
				default:
					return "", 0, errors.New("unexpected git " + args[0])
				}
			}
			cs := CycleState{CycleID: unwindCycle, RunID: unwindRunID, ActiveWorktree: worktree}

			err := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)).healCarrierAtHead(context.Background(), t.TempDir(), unwindCycle, cs, git)

			if want := fmt.Sprintf(tc.want, worktree); err == nil || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}

func TestVerifyResumeIdentityAndHealCarrier_AnUnnumberedCycleStateHealsTheCheckpointCyclesCarrier(t *testing.T) {
	t.Parallel()
	fx := rebasedCarrierLane(t)
	cs := fx.cycleState()
	cs.CycleID = 0
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	if err := o.verifyResumeIdentityAndHealCarrier(context.Background(), fx.root, &ResumePoint{CycleID: unwindCycle}, cs); err != nil {
		t.Fatalf("verifyResumeIdentityAndHealCarrier: %v", err)
	}

	fx.requirePendingLaneChangeOn(fx.newBase)
}

func TestLoadResumeBootstrap_ACarrierThatCannotBePendedFailsTheResume(t *testing.T) {
	t.Parallel()
	fx := rebasedCarrierLane(t)
	fx.git("branch", "-m", "main", "trunk")
	o := NewOrchestrator(&fakeStorage{cycleState: fx.cycleState()}, &fakeLedger{}, buildRunners(nil))

	_, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: fx.root}, &ResumePoint{CycleID: unwindCycle}, PhaseShip)

	if err == nil || !strings.HasPrefix(err.Error(), "heal a carrier left at HEAD: pend the carrier cycle-42/run-42: ") {
		t.Fatalf("err = %v, want the pend failure", err)
	}
}

func TestLoadResumeBootstrap_LeavesACarrierAloneWhileARebaseIsInProgress(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"rebase-merge", "rebase-apply"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			fx := rebasedCarrierLane(t)
			head := fx.git("rev-parse", "HEAD")
			gitPath := fx.git("rev-parse", "--git-path", state)
			if !filepath.IsAbs(gitPath) {
				gitPath = filepath.Join(fx.worktree, gitPath)
			}
			if err := os.MkdirAll(gitPath, 0o755); err != nil {
				t.Fatal(err)
			}
			var sink injectedStderr
			o := NewOrchestrator(&fakeStorage{cycleState: fx.cycleState()}, &fakeLedger{}, buildRunners(nil), WithStderr(&sink))

			if _, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: fx.root}, &ResumePoint{CycleID: unwindCycle}, PhaseShip); err != nil {
				t.Fatalf("loadResumeBootstrap: %v", err)
			}

			if got := fx.git("rev-parse", "HEAD"); got != head {
				t.Fatalf("HEAD moved from %s to %s during a rebase", head, got)
			}
			if !strings.Contains(sink.String(), "a rebase is in progress") {
				t.Fatalf("stderr does not say why the carrier stays:\n%s", sink.String())
			}
		})
	}
}

func TestVerifyResumeIdentityAndHealCarrier_ACheckpointForAnotherWorktreeIsRejectedBeforeAnyHeal(t *testing.T) {
	t.Parallel()
	fx := rebasedCarrierLane(t)
	head := fx.git("rev-parse", "HEAD")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	err := o.verifyResumeIdentityAndHealCarrier(context.Background(), fx.root, &ResumePoint{CycleID: unwindCycle, WorktreePath: t.TempDir()}, fx.cycleState())

	if err == nil || !strings.HasPrefix(err.Error(), "resume identity mismatch: cycle-state worktree ") {
		t.Fatalf("err = %v, want the worktree identity mismatch", err)
	}
	if got := fx.git("rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD moved from %s to %s on a mismatched resume", head, got)
	}
}

func TestIsGitCheckout_AnUnreadableLaneIsAnErrorNotASkip(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	worktree := filepath.Join(parent, "lane")
	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(worktree, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(worktree, 0o755) })

	checkout, err := isGitCheckout(worktree)

	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 directory")
	}
	if checkout || err == nil || !strings.Contains(err.Error(), "read the git checkout of lane "+worktree) {
		t.Fatalf("isGitCheckout = (%v, %v), want the stat error", checkout, err)
	}
}
