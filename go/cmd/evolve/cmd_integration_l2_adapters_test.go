package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
)

func l2TipRepo(t *testing.T) (integration.Repo, string) {
	t.Helper()
	g := gittest.Fixture(t)
	g.Write("go/go.mod", "module fixture\n\ngo 1.23\n")
	g.Write("go/p/p.go", "package p\n")
	g.Write("go/p/p_test.go", "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n")
	g.Write("go/acs/cycle9/predicates_test.go", "package cycle9\n\nimport \"testing\"\n\nfunc TestC9_001(t *testing.T) {}\n")
	g.Git("add", "-A")
	g.Git("commit", "-q", "-m", "fixture")
	tip := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
	return integration.Repo{Dir: g.Dir, Git: integration.SysGit()}, tip
}

func TestL2Adapters_RunAGoTestAPredicateRunAndAnAffectedSetOnACheckedOutTip(t *testing.T) {
	t.Parallel()
	repo, tip := l2TipRepo(t)
	checkout := l2Checkout(repo, filepath.Join(repo.Dir, ".evolve"))
	ctx := context.Background()

	set, err := l2Affected(checkout)(tip, []string{"./p"})
	if err != nil || len(set) == 0 {
		t.Fatalf("l2Affected = %v, %v, want the package p", set, err)
	}
	if none, err := l2Affected(checkout)(tip, nil); err != nil || none != nil {
		t.Errorf("l2Affected of no edit = %v, %v, want nothing", none, err)
	}

	out, err := l2PackageTest(checkout, sysL2Command)(ctx, tip, "./p")
	if err != nil || !out.Green {
		t.Fatalf("l2PackageTest = %+v, %v, want green", out, err)
	}
	red, err := l2PackageTest(checkout, sysL2Command)(ctx, tip, "./missing")
	if err != nil || red.Green {
		t.Errorf("l2PackageTest of a missing package = %+v, %v, want a red outcome", red, err)
	}

	v, err := l2Predicates(checkout)(ctx, tip, 9)
	if err != nil || v.RedCount != 0 {
		t.Errorf("l2Predicates = %+v, %v, want green", v, err)
	}
	if _, err := l2Predicates(checkout)(ctx, "deadbeef", 9); err == nil {
		t.Error("l2Predicates on a missing tip must fail")
	}
	if _, err := l2Affected(checkout)("deadbeef", []string{"./p"}); err == nil {
		t.Error("l2Affected on a missing tip must fail")
	}
	if _, err := l2PackageTest(checkout, sysL2Command)(ctx, "deadbeef", "./p"); err == nil {
		t.Error("l2PackageTest on a missing tip must fail")
	}
	if entries, _ := os.ReadDir(filepath.Join(repo.Dir, ".evolve", l2CheckoutDir)); len(entries) != 0 {
		t.Errorf("checkouts left behind: %d", len(entries))
	}
}

func TestL2Adapters_ACommandThatCannotStartIsAHostFailureAndAKilledOneIsNamed(t *testing.T) {
	t.Parallel()
	repo, tip := l2TipRepo(t)
	checkout := l2Checkout(repo, filepath.Join(repo.Dir, ".evolve"))
	broken := func(context.Context, string, ...string) (string, int, error) { return "", 0, errors.New("no go") }
	if _, err := l2PackageTest(checkout, broken)(context.Background(), tip, "./p"); !acsrunner.IsHostFailure(err) {
		t.Errorf("l2PackageTest = %v, want a host failure", err)
	}
	if out, code, err := sysL2Command(context.Background(), t.TempDir(), "sh", "-c", "echo hi"); err != nil || code != 0 || !strings.Contains(out, "hi") {
		t.Errorf("sysL2Command = %q, %d, %v", out, code, err)
	}
	if _, code, err := sysL2Command(context.Background(), t.TempDir(), "sh", "-c", "kill -9 $$"); !errors.Is(err, errL2CommandKilled) || code != -1 {
		t.Errorf("sysL2Command of a killed command = %d, %v, want the killed error", code, err)
	}
}

func TestL2Adapters_TheStageFallsBackToShadowAndTheGateAndVerdictsAnswer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeL2Policy(t, root, `{"fleet":{"integration":{"stage":"bogus"}}}`)
	var warn strings.Builder
	if got := l2Stage(root, &warn); got != "shadow" || !strings.Contains(warn.String(), "stays at shadow") {
		t.Errorf("l2Stage = %q, %q, want shadow with a warning", got, warn.String())
	}
	writeL2Policy(t, root, enforceL2PolicyJSON)
	if got := l2Stage(root, io.Discard); got != "enforce" {
		t.Errorf("l2Stage = %q, want enforce", got)
	}
	if err := l2NoGate(context.Background(), "", *new(overlap.Tier)); err != nil {
		t.Errorf("l2NoGate = %v", err)
	}
	v := &tipVerdicts{green: map[string]bool{}}
	if v.Green("t", "p") {
		t.Error("an unrecorded tip is not green")
	}
	if err := v.Record("t", "p"); err != nil || !v.Green("t", "p") {
		t.Errorf("Record then Green = %v", err)
	}
}

func TestL2Adapters_ACheckoutDirThatCannotBeMadeIsAnError(t *testing.T) {
	t.Parallel()
	repo, tip := l2TipRepo(t)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := l2Checkout(repo, blocker)(context.Background(), tip); err == nil {
		t.Error("a checkout under a file must fail")
	}
	evolve := t.TempDir()
	base := filepath.Join(evolve, l2CheckoutDir)
	if err := os.Mkdir(base, 0o500); err != nil {
		t.Fatal(err)
	}
	if _, _, err := l2Checkout(repo, evolve)(context.Background(), tip); err == nil && os.Geteuid() != 0 {
		t.Error("a checkout in a read-only directory must fail")
	}
}
