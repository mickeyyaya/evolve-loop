package commitgate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func repoContractRun(t *testing.T, changed string, reviewers string, step RepoContractStep) (*Result, string) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
	o := baseOpts(root, "shasum", "go")
	o.Reviewers = reviewers
	o.Env = os.Environ()
	o.RepoContract = step
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: changed + "\n"},
		{matchPrefix: "git diff HEAD", stdout: "diff\n", exit: 1},
	}}
	o.Runner = sr.run()
	return o.Run(context.Background()), root
}

func TestRun_ARedRepoContractPackRefusesTheCommitAndNamesTheFailingTests(t *testing.T) {
	t.Parallel()
	const red = "example.com/x/internal/testaudit.TestTreeRatchet"
	var ranIn []string
	res, root := repoContractRun(t, "x.go", "code-simplifier,code-reviewer", func(_ context.Context, dir string) ([]string, error) {
		ranIn = append(ranIn, dir)
		return []string{"repo-contract scanner pack RED: " + red}, nil
	})
	if res.ExitCode != ExitFail || res.Attestation != nil {
		t.Fatalf("ExitCode = %d, attestation %v; want ExitFail and no attestation (%v)", res.ExitCode, res.Attestation, res.Logs)
	}
	if !strings.Contains(strings.Join(res.Logs, "\n"), red) {
		t.Errorf("logs %v do not name the failing test %s", res.Logs, red)
	}
	if !reflect.DeepEqual(ranIn, []string{root}) {
		t.Errorf("the pack ran in %v, want once in the repository root %s", ranIn, root)
	}
}

func TestRun_AnUnnamedRepoContractPackFaultRefusesTheCommit(t *testing.T) {
	t.Parallel()
	res, _ := repoContractRun(t, "x.go", "code-simplifier,code-reviewer", func(context.Context, string) ([]string, error) {
		return nil, errors.New("scanner pack exited nonzero naming no test")
	})
	if res.ExitCode != ExitFail || !strings.Contains(strings.Join(res.Logs, "\n"), "naming no test") {
		t.Fatalf("ExitCode = %d, logs %v; want ExitFail and the fault text", res.ExitCode, res.Logs)
	}
}

func TestRun_AGreenRepoContractPackIsRecordedAsACheck(t *testing.T) {
	t.Parallel()
	res, _ := repoContractRun(t, "x.go", "code-simplifier,code-reviewer", func(context.Context, string) ([]string, error) { return nil, nil })
	if res.ExitCode != ExitPass || !reflect.DeepEqual(res.ChecksPassed, []string{"go:gofmt", "go:vet", "go:test", "go:repo-contract"}) {
		t.Fatalf("ExitCode = %d, checks %v, logs %v", res.ExitCode, res.ChecksPassed, res.Logs)
	}
}

func TestRun_AStagedDocsOnlyChangeRunsNoRepoContractPack(t *testing.T) {
	t.Parallel()
	called := false
	o := docsGate(t, "docs-review")
	o.RepoContract = func(context.Context, string) ([]string, error) {
		called = true
		return []string{"RED"}, nil
	}
	res := o.Run(context.Background())
	if called || res.ExitCode != ExitPass {
		t.Fatalf("pack called = %v, ExitCode = %d (%v); a docs-only change has no Go lane and runs no pack", called, res.ExitCode, res.Logs)
	}
}

func TestRun_APackThatOutlivesItsDeadlineRefusesAndSaysItDidNotFinish(t *testing.T) {
	t.Parallel()
	fault := &core.PackDeadlineError{Stage: core.CommitGateStage(), Cause: context.DeadlineExceeded, Elapsed: 120500 * time.Millisecond}
	res, _ := repoContractRun(t, "x.go", "code-simplifier,code-reviewer", func(context.Context, string) ([]string, error) { return nil, fault })
	log := strings.Join(res.Logs, "\n")
	if res.ExitCode != ExitFail || res.Attestation != nil {
		t.Fatalf("a timeout fails closed; ExitCode = %d (%v)", res.ExitCode, res.Logs)
	}
	if !strings.Contains(log, fault.Stage.Rerun) {
		t.Errorf("the refusal carries the stage's re-run advice; logs %q", log)
	}
}
