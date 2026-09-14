package ship

// repocontract_importers_test.go — the importer backstop and the gate's seed.
// A lane's own package tests are green, but a package that IMPORTS what the
// lane changed asserts the old contract and goes red on main's CI (cycle
// 1657/1659: the lane renamed a stop in internal/core; internal/deliverable's
// e2e test, untouched, redded main from 4db205a8 until #590). The gate now
// runs the reverse-dependency closure of every changed package — read from
// the WORKING TREE of the tree that will land, never from an index the ship
// has not yet populated — before a ship may land.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// importerFixture is a module with base ← user (user's non-test code imports
// base; its test asserts base's contract), base ← xonly (ONLY xonly's external
// test imports base), and an unrelated other; the four guard suites are green
// stubs. Everything is committed: the tests below change the tree the way a
// lane does — unstaged edits and untracked files.
func importerFixture(t *testing.T) (repo, goDir string) {
	t.Helper()
	repo = makeRepo(t)
	goDir = filepath.Join(repo, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
	for _, pkg := range []string{"phasespec", "profiles", "phasecoherence", "routingtest"} {
		mustWrite(t, filepath.Join(goDir, "internal", pkg, "pass_test.go"), "package "+pkg+"\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n")
	}
	mustWrite(t, filepath.Join(goDir, "internal", "base", "base.go"), "package base\n\nconst Stop = \"old-stop\"\n")
	mustWrite(t, filepath.Join(goDir, "internal", "user", "user.go"), "package user\n\nimport \"example.com/lane/internal/base\"\n\nfunc Stop() string { return base.Stop }\n")
	mustWrite(t, filepath.Join(goDir, "internal", "user", "user_test.go"), "package user\n\nimport \"testing\"\n\nfunc TestUserContract(t *testing.T) {\n\tif Stop() != \"old-stop\" {\n\t\tt.Fatalf(\"contract: %q\", Stop())\n\t}\n}\n")
	mustWrite(t, filepath.Join(goDir, "internal", "xonly", "doc.go"), "package xonly\n")
	mustWrite(t, filepath.Join(goDir, "internal", "xonly", "x_test.go"), "package xonly_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/lane/internal/base\"\n)\n\nfunc TestXOnly(t *testing.T) { _ = base.Stop }\n")
	mustWrite(t, filepath.Join(goDir, "internal", "other", "other.go"), "package other\n\nconst Unrelated = 1\n")
	mustWrite(t, filepath.Join(goDir, "internal", "other", "other_test.go"), "package other\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) {}\n")
	runGit(t, repo, "add", "go")
	runGit(t, repo, "commit", "-qm", "baseline: base ← user, base ← xonly (test only), other, green guard suites")
	return repo, goDir
}

// The incident shape on the cycle path: the lane changes base — UNSTAGED —
// its own tests are green, user's untouched test asserts the old contract.
// The gate goes RED naming it, and the test-only importer is in the closure.
func TestRepoContractGate_ImporterOfAChangedPackageBlocksShip(t *testing.T) {
	repo, goDir := importerFixture(t)
	mustWrite(t, filepath.Join(goDir, "internal", "base", "base.go"), "package base\n\nconst Stop = \"new-stop\"\n")
	var out strings.Builder
	err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), &out)
	if err == nil {
		t.Fatal("a red importer must block the ship before it can red main")
	}
	se, ok := shiperr.AsShipError(err)
	if !ok || se.Code != shiperr.CodeRepoContractGate {
		t.Fatalf("error = %v, want REPO_CONTRACT_GATE", err)
	}
	if !strings.Contains(err.Error(), "TestUserContract") || !strings.Contains(err.Error(), "importer backstop") {
		t.Fatalf("the gate names the layer and the importer's failing test: %q", err)
	}
	wantRun := fmt.Sprintf("[ship] repo-contract importer backstop: go test -json -count=1 -timeout %s ./internal/base/... ./internal/user/... ./internal/xonly/...", repoContractTestTimeout)
	if !strings.Contains(out.String(), wantRun) {
		t.Errorf("the backstop runs the changed package, its importer and its test-only importer: %q", out.String())
	}
}

// A change nothing imports runs only its own package — the closure is bounded.
func TestRepoContractGate_UnimportedChangeRunsOnlyItself(t *testing.T) {
	repo, goDir := importerFixture(t)
	mustWrite(t, filepath.Join(goDir, "internal", "other", "other.go"), "package other\n\nconst Unrelated = 2\n")
	var out strings.Builder
	if err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), &out); err != nil {
		t.Fatalf("an unrelated change ships: %v", err)
	}
	if strings.Contains(out.String(), "./internal/user") {
		t.Errorf("user does not import other and must not run: %q", out.String())
	}
	wantRun := fmt.Sprintf("importer backstop: go test -json -count=1 -timeout %s ./internal/other/... (1 changed → 1 in closure", repoContractTestTimeout)
	if !strings.Contains(out.String(), wantRun) {
		t.Errorf("the changed package itself runs: %q", out.String())
	}
}

// No Go change (a docs-only ship): no backstop run, said once.
func TestRepoContractGate_NoGoChangeSkipsTheImporterBackstop(t *testing.T) {
	repo, _ := importerFixture(t)
	mustWrite(t, filepath.Join(repo, "docs", "only.md"), "docs\n")
	var out strings.Builder
	if err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), &out); err != nil {
		t.Fatalf("docs-only ships: %v", err)
	}
	if !strings.Contains(out.String(), "importer backstop: no Go change in the tree — skipped") {
		t.Errorf("the skip is said: %q", out.String())
	}
}

// The cycle path for the added-test backstop: a lane's new red test is
// UNTRACKED at gate time (the ship stages it later). An index-based seed saw
// nothing here and let the reproducer red main.
func TestRepoContractGate_UntrackedAddedRedTestBlocksShip(t *testing.T) {
	repo, goDir := importerFixture(t)
	mustWrite(t, filepath.Join(goDir, "internal", "reproduction", "red_test.go"), "package reproduction\n\nimport \"testing\"\n\nfunc TestUntrackedRed(t *testing.T) { t.Fatal(\"deliberate red\") }\n")
	err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "TestUntrackedRed") || !strings.Contains(err.Error(), "added-test backstop") {
		t.Fatalf("an untracked red test blocks the ship via the added-test backstop, got %v", err)
	}
}

// A tag-only package seeds the walk but is said, not run.
func TestRepoContractGate_TagOnlyChangeIsSaidNotRun(t *testing.T) {
	repo, goDir := importerFixture(t)
	mustWrite(t, filepath.Join(goDir, "acs", "cycle9999", "predicates_test.go"), "//go:build acs\n\npackage cycle9999\n\nimport \"testing\"\n\nfunc TestHealthy(t *testing.T) {}\n")
	var out strings.Builder
	if err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), &out); err != nil {
		t.Fatalf("a tag-only package ships: %v", err)
	}
	if !strings.Contains(out.String(), "not run: ./acs/cycle9999/...") {
		t.Errorf("the tag-only pattern is said, not run: %q", out.String())
	}
}

// Discovery failure is a re-dispatchable INFRA class, never a silent green:
// the fixed pack is green (seam), but the root is not a repository.
func TestRepoContractGate_DiscoveryFailureIsInfra(t *testing.T) {
	swapRepoContractTest(t, greenPack())
	err := runRepoContractGate(context.Background(), "enforce", t.TempDir(), t.TempDir(), io.Discard)
	se, ok := shiperr.AsShipError(err)
	if !ok || se.Code != shiperr.CodeRepoContractInfra {
		t.Fatalf("error = %v, want REPO_CONTRACT_INFRA", err)
	}
	if !strings.Contains(err.Error(), "backstop discovery") {
		t.Errorf("the infra error says what could not be derived: %v", err)
	}
}

// The wiring proof: a cycle ship's changes live in the LANE WORKTREE, and
// the gate must test that tree against the worktree's base. The project root
// (main's pre-landing tree) is untouched here — a gate that ran there, as it
// did until 2026-09-14, passes green and lets the red importer land.
func TestRunNative_GateTestsTheLaneWorktreeNotTheProjectRoot(t *testing.T) {
	repo, _ := importerFixture(t)
	base := runGitOut(t, repo, "rev-parse", "HEAD")
	wt := filepath.Join(t.TempDir(), "cycle-1673")
	runGit(t, repo, "worktree", "add", "--detach", "-q", wt, "HEAD")
	mustWrite(t, filepath.Join(wt, "go", "internal", "base", "base.go"), "package base\n\nconst Stop = \"new-stop\"\n")

	ws := t.TempDir()
	p := New(Config{RepoContractGate: "enforce"})
	_, err := p.runNative(context.Background(), core.PhaseRequest{
		Cycle: 1673, Workspace: ws, ProjectRoot: repo, Worktree: wt, WorktreeBaseSHA: strings.TrimSpace(base),
	}, "msg", time.Now())
	if err == nil {
		t.Fatal("the gate must test the lane worktree: its red importer blocks the ship")
	}
	if !strings.Contains(err.Error(), "repo-contract gate") || !strings.Contains(err.Error(), "TestUserContract") {
		t.Fatalf("block must come from the repo-contract gate on the worktree's tree, got %v", err)
	}
	scan, _ := os.ReadFile(filepath.Join(ws, scanLogName))
	if !strings.Contains(string(scan), "(module "+filepath.Join(wt, "go")+", changes vs "+strings.TrimSpace(base)+")") {
		t.Errorf("the scan log names the worktree module and the base: %s", scan)
	}
}

// An unresolvable typed worktree fails the gate closed with the ship's own
// worktree-resolve class — one landing-tree decision (landingTree) serves
// atomicShip and the gate, so the gate never tests the project root in the
// lane's stead and reports a misleading green before Run aborts.
func TestRepoContractGateRoot_UnresolvableTypedWorktreeFailsClosed(t *testing.T) {
	repo, _ := importerFixture(t)
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, core.RunStateFile), `{"active_worktree":"/elsewhere/cycle-1"}`+"\n")
	opts := Options{Class: ClassCycle, ProjectRoot: repo, WorkspacePath: ws, ActiveWorktree: filepath.Join(t.TempDir(), "cycle-1")}
	_, _, err := repoContractGateRoot(&opts)
	se, ok := shiperr.AsShipError(err)
	if !ok || se.Code != shiperr.CodeWorktreeResolve {
		t.Fatalf("mirror mismatch: err = %v, want WORKTREE_RESOLVE", err)
	}
	opts = Options{Class: ClassCycle, ProjectRoot: repo, ActiveWorktree: filepath.Join(t.TempDir(), "gone")}
	if _, _, err := repoContractGateRoot(&opts); err == nil || !strings.Contains(err.Error(), "is unavailable") {
		t.Fatalf("a typed worktree that is not on disk: err = %v", err)
	}
	opts = Options{Class: ClassManual, ProjectRoot: repo}
	if root, base, err := repoContractGateRoot(&opts); err != nil || root != repo || base != "HEAD" {
		t.Fatalf("manual class: root=%q base=%q err=%v", root, base, err)
	}
}
