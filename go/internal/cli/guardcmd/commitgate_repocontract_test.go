package guardcmd

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

func stagedGoChangeInEvolveModule(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("go/go.mod", "module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.23\n")
	repo.Write("go/x/x.go", "package x\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-qm", "base")
	repo.Write("go/x/y.go", "package x\n")
	repo.Git("add", "-A")
	return repo.Dir
}

func fakePack(t *testing.T, reds []string, err error) *[]string {
	t.Helper()
	var ranIn []string
	prev := repoContractPack
	t.Cleanup(func() { repoContractPack = prev })
	repoContractPack = func(_ context.Context, _, root string, _ []string) ([]string, string, error) {
		ranIn = append(ranIn, root)
		return reds, "", err
	}
	return &ranIn
}

func TestRepoContractPackSeam_DefaultsToShipsOwnPack(t *testing.T) {
	t.Parallel()
	if got, want := reflect.ValueOf(repoContractPack).Pointer(), reflect.ValueOf(ship.RunRepoContractPack).Pointer(); got != want {
		t.Fatal("the commit gate must run ship's own repo-contract pack, so the suite list has one home")
	}
}

func TestRunCommitGate_ARedRepoContractPackRefusesAGoChangeAndNamesTheTest(t *testing.T) {
	const red = "github.com/mickeyyaya/evolve-loop/go/internal/testaudit.TestTreeRatchet"
	repo := stagedGoChangeInEvolveModule(t)
	t.Setenv("CG_ATTEST_DIR", t.TempDir())
	ranIn := fakePack(t, []string{red}, errors.New("exit status 1"))
	var out, errb bytes.Buffer

	rc := RunCommitGate([]string{"run", "--project-root", repo, "--reviewers", "code-simplifier,code-reviewer", "--no-install"}, nil, &out, &errb)

	if rc != commitgate.ExitFail || !strings.Contains(errb.String(), red) || out.Len() != 0 {
		t.Fatalf("rc %d stdout %q stderr %q: want ExitFail, the red test named and no attested tree", rc, out.String(), errb.String())
	}
	if len(*ranIn) != 1 {
		t.Errorf("the pack ran %d times, want once", len(*ranIn))
	}
	var refusal string
	for _, line := range strings.Split(errb.String(), "\n") {
		if strings.HasPrefix(line, "[commit-gate] go: repo-contract scanner pack RED") {
			refusal = line
		}
	}
	if refusal == "" {
		t.Fatalf("the refusal speaks as the commit gate; stderr %q", errb.String())
	}
	for _, banned := range []string{"ship runs", "build-floor", "handoff"} {
		if strings.Contains(refusal, banned) {
			t.Errorf("the commit gate's refusal must not say %q; got %q", banned, refusal)
		}
	}
}

func TestRunCommitGate_AStagedDocsOnlyChangeRunsNoRepoContractPack(t *testing.T) {
	repo := stagedDocsRepo(t)
	t.Setenv("CG_ATTEST_DIR", t.TempDir())
	ranIn := fakePack(t, []string{"RED"}, errors.New("exit status 1"))
	var out, errb bytes.Buffer

	rc := RunCommitGate([]string{"run", "--project-root", repo, "--reviewers", "docs-review", "--no-install"}, nil, &out, &errb)

	if rc != commitgate.ExitPass || len(*ranIn) != 0 {
		t.Fatalf("rc %d, pack ran in %v, stderr %q: a docs-only change passes and runs no pack", rc, *ranIn, errb.String())
	}
}
