package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const l2CheckoutDir = "l2-checkouts"

var errL2CommandKilled = errors.New("the batch test command was killed by a signal")

func sysL2Command(ctx context.Context, dir string, argv ...string) (string, int, error) {
	tail := newTailWriter(64 * 1024)
	code, err := sysexec.DefaultRunner(ctx, argv[0], dir, argv[1:], lanebudget.Env(ciparity.CIEnv(os.Environ())), nil, tail, tail)
	if err == nil && code == -1 {
		err = errL2CommandKilled
	}
	return tail.String(), code, err
}

func (e integrationEnv) batchTest(repo integration.Repo, projectRoot string, warn io.Writer) integration.BatchTest {
	checkout := l2Checkout(repo, paths.EvolveDirOf(projectRoot))
	return integration.BatchTest{
		Composer:   integration.Composer{Repo: repo, Stage: l2Stage(projectRoot, warn), Gate: l2NoGate},
		Affected:   l2Affected(checkout),
		Test:       l2PackageTest(checkout, e.Run),
		Predicates: l2Predicates(checkout),
		Verdicts:   &tipVerdicts{green: map[string]bool{}},
		Pack:       integration.L2Pack(checkout, e.Run),
	}
}

func l2Stage(projectRoot string, warn io.Writer) string {
	stage, err := policy.LoadOrDefaults(filepath.Join(paths.EvolveDirOf(projectRoot), "policy.json"), warn).FleetIntegrationStage()
	if err != nil {
		fmt.Fprintf(warn, "%sthe batch test stage is invalid, so it stays at shadow: %v\n", integrationPrefix, err)
		return policy.FleetIntegrationShadow
	}
	return stage
}

func l2NoGate(context.Context, string, overlap.Tier) error { return nil }

func l2Checkout(repo integration.Repo, evolveDir string) integration.Checkout {
	return func(ctx context.Context, tip string) (string, func(), error) {
		base := filepath.Join(evolveDir, l2CheckoutDir)
		if err := os.MkdirAll(base, 0o755); err != nil {
			return "", nil, err
		}
		dir, err := os.MkdirTemp(base, "tip-")
		if err != nil {
			return "", nil, err
		}
		if _, err := repo.Git(ctx, repo.Dir, "", "worktree", "add", "--detach", dir, tip); err != nil {
			_ = os.RemoveAll(dir)
			return "", nil, err
		}
		release := func() {
			_, _ = repo.Git(context.Background(), repo.Dir, "", "worktree", "remove", "--force", dir)
			_ = os.RemoveAll(dir)
		}
		return dir, release, nil
	}
}

func l2Affected(checkout integration.Checkout) func(string, []string) ([]string, error) {
	return func(tip string, edited []string) ([]string, error) {
		if len(edited) == 0 {
			return nil, nil
		}
		dir, release, err := checkout(context.Background(), tip)
		if err != nil {
			return nil, err
		}
		defer release()
		return integration.AffectedSet(dir, edited)
	}
}

func l2PackageTest(checkout integration.Checkout, run integration.Command) integration.PackageTest {
	return func(ctx context.Context, tip, pkg string) (integration.TestOutcome, error) {
		dir, release, err := checkout(ctx, tip)
		if err != nil {
			return integration.TestOutcome{}, err
		}
		defer release()
		out, exit, err := run(ctx, filepath.Join(dir, "go"), integration.PackageTestArgv(pkg)...)
		if err != nil {
			return integration.TestOutcome{}, acsrunner.HostFailure(err)
		}
		return integration.TestOutcome{Green: exit == 0, Output: out}, nil
	}
}

func l2Predicates(checkout integration.Checkout) integration.PredicateRun {
	return func(ctx context.Context, tip string, cycle int) (acsrunner.Verdict, error) {
		dir, release, err := checkout(ctx, tip)
		if err != nil {
			return acsrunner.Verdict{}, err
		}
		defer release()
		return acsrunner.RunIn(ctx, filepath.Join(dir, "go"), cycle, "./acs/cycle"+strconv.Itoa(cycle))
	}
}

type tipVerdicts struct {
	mu    sync.Mutex
	green map[string]bool
}

func (v *tipVerdicts) Green(tip, pkg string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.green[tip+" "+pkg]
}

func (v *tipVerdicts) Record(tip, pkg string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.green[tip+" "+pkg] = true
	return nil
}
