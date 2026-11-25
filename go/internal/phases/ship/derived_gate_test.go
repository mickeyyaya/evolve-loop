//go:build integration

package ship

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/researchindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func checkRefused(t *testing.T, output string) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit 3").Run()
	if err == nil {
		t.Fatal("fixture exit must fail")
	}
	return fmt.Errorf("%w: %s", err, output)
}

func derivedShipOptions(t *testing.T, repo string, run derived.Generator) *Options {
	t.Helper()
	opts := auditOpts(t, repo)
	opts.Class, opts.Stderr, opts.derivedRun = ClassCycle, io.Discard, run
	return opts
}

func shipErrOf(t *testing.T, err error) *core.ShipError {
	t.Helper()
	var se *core.ShipError
	if !errors.As(err, &se) {
		t.Fatalf("err = %v, want a ShipError", err)
	}
	return se
}

func shipCode(t *testing.T, err error) (shiperr.ShipErrorCode, core.ShipErrorClass) {
	t.Helper()
	se := shipErrOf(t, err)
	return se.Code, se.Class
}

func repoWithAnEntry(t *testing.T, body string) string {
	t.Helper()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, researchindex.EntriesDir, "2099-01-01-01-e2e.md"), body)
	return repo
}

func TestShip_CycleRefusesAResearchEntryThatStartsWithABlankLine(t *testing.T) {
	t.Parallel()
	repo := repoWithAnEntry(t, "\n## 2099-01-01 — blank first line\n")
	run := func(_ context.Context, args ...string) error {
		return checkRefused(t, `entry 2099-01-01-01-e2e.md must start with a "## " heading line`)
	}
	seedAudit(t, repo, "PASS")
	var res RunResult

	err := verifyClass(context.Background(), derivedShipOptions(t, repo, run), &res)

	code, class := shipCode(t, err)
	if code != shiperr.CodeDerivedStale || class != core.ShipClassPrecondition || !strings.Contains(shipErrOf(t, err).Message, "2099-01-01-01-e2e.md") {
		t.Fatalf("err = %v (%s/%s), want DERIVED_STALE precondition that names the entry file", err, code, class)
	}
}

func TestShip_CycleRefusesAValidNewEntryWithAStaleIndexAndNamesTheGenerateVerb(t *testing.T) {
	t.Parallel()
	repo := repoWithAnEntry(t, "## 2099-01-01 — a valid entry\n")
	run := func(_ context.Context, args ...string) error {
		return checkRefused(t, "docs/research-index.md is stale")
	}
	seedAudit(t, repo, "PASS")
	var res RunResult

	err := verifyClass(context.Background(), derivedShipOptions(t, repo, run), &res)

	if code, _ := shipCode(t, err); code != shiperr.CodeDerivedStale || !strings.Contains(shipErrOf(t, err).Message, "evolve docs index generate") {
		t.Fatalf("err = %v, want DERIVED_STALE naming `evolve docs index generate`", err)
	}
}

func TestShip_CycleShipsWhenTheDerivedFilesAreInSync(t *testing.T) {
	t.Parallel()
	repo := repoWithAnEntry(t, "## 2099-01-01 — a valid entry\n")
	var ran []string
	run := func(_ context.Context, args ...string) error { ran = append(ran, strings.Join(args, " ")); return nil }
	seedAudit(t, repo, "PASS")
	var res RunResult

	err := verifyClass(context.Background(), derivedShipOptions(t, repo, run), &res)

	if err != nil || len(ran) != 1 || ran[0] != "docs index check" {
		t.Fatalf("err = %v, checks = %v, want a pass that ran only the research-index check", err, ran)
	}
}

func TestShip_ACheckThatCannotRunIsATransientRefusalNotAPass(t *testing.T) {
	t.Parallel()
	repo := repoWithAnEntry(t, "## 2099-01-01 — a valid entry\n")
	run := func(context.Context, ...string) error { return errors.New("go: not found") }
	seedAudit(t, repo, "PASS")
	var res RunResult

	err := verifyClass(context.Background(), derivedShipOptions(t, repo, run), &res)

	if code, class := shipCode(t, err); code != shiperr.CodeDerivedStale || class != core.ShipClassTransient {
		t.Fatalf("err = %v (%s/%s), want a transient DERIVED_STALE refusal", err, code, class)
	}
}

func TestShip_AWorktreeTreeThatCannotBeResolvedRefuses(t *testing.T) {
	t.Parallel()
	opts := derivedShipOptions(t, makeRepo(t), nil)
	opts.ActiveWorktree = filepath.Join(t.TempDir(), "missing")
	opts.Env = map[string]string{}

	err := refuseStaleDerived(context.Background(), opts, &RunResult{})

	if code, _ := shipCode(t, err); code != shiperr.CodeWorktreeResolve {
		t.Fatalf("err = %v, want the worktree resolution refusal", err)
	}
}

func TestShip_TheRealGeneratorRefusesABlankLeadingEntryOfACloneAndPassesACleanOne(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=readonly")
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	clone := gittest.Clone(t, root)
	opts := &Options{ProjectRoot: clone.Dir, Class: ClassCycle, Runner: execRunner, Stderr: io.Discard, derivedRun: core.LaneDerivedGenerator(clone.Dir)}
	if err := refuseStaleDerived(context.Background(), opts, &RunResult{}); err != nil {
		t.Fatalf("a clean clone of HEAD must pass: %v", err)
	}
	clone.Write(researchindex.EntriesDir+"/2099-01-01-01-e2e.md", "\n## 2099-01-01 — blank first line\n")

	err = refuseStaleDerived(context.Background(), opts, &RunResult{})

	if code, _ := shipCode(t, err); code != shiperr.CodeDerivedStale || !strings.Contains(shipErrOf(t, err).Message, "2099-01-01-01-e2e.md") {
		t.Fatalf("err = %v, want DERIVED_STALE that names the entry file", err)
	}
}

func TestShip_ATreeWhoseChangesGitCannotDeriveIsRefusedAsInfra(t *testing.T) {
	t.Parallel()
	opts := &Options{ProjectRoot: t.TempDir(), Class: ClassCycle, Runner: execRunner, Stderr: io.Discard}

	err := refuseStaleDerived(context.Background(), opts, &RunResult{})

	if code, _ := shipCode(t, err); code != shiperr.CodeRepoContractInfra {
		t.Fatalf("err = %v, want the change-discovery INFRA refusal, never a pass", err)
	}
}
