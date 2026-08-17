//go:build acs

// Package cycle1518 materialises the cycle-1518 acceptance criteria for the
// single triage-committed task
// `promote-cycle1515-continuation-predicates-to-acs-regression`.
//
// Scope note (verified live, not assumed from the filed item). This lane's
// fleet_scope.todo_ids — `registry-release-on-park-consume` and
// `continuation-operator-cli` — are BOTH already implemented and green at this
// worktree's HEAD (`internal/inboxmover/continuation_retire.go`,
// `internal/phases/ship/consume.go`, `go/cmd/evolve/cmd_continuation.go` wired
// at `registry.go:73`). The scout/triage verdict is therefore a durability gap,
// not a functionality gap: the eight predicates that pin that shipped behaviour
// live in `go/acs/cycle1515/` — a PER-CYCLE ACS package that CI never sweeps.
// `.github/workflows/ci.yml`'s `acs-durable` job runs exactly
// `go test -count=1 -tags acs ./acs/regression/...`, so nothing under
// `go/acs/cycle1515/` is a standing gate. This cycle promotes that file into
// the durable path.
//
// Destination pinned by these predicates: `go/acs/regression/cycle1515/`,
// package `cycle1515`, with the eight `TestC1515_001..008` names carried over
// VERBATIM. Triage's files list illustrated the destination as
// `go/acs/regression/continuation_operator_cli/`; that path is NOT used, for two
// reasons stated here rather than decided silently: (1) fifteen sibling durable
// packages already use the `regression/cycle<N>` shape (cycle100, cycle1270,
// cycle85 …) and none uses an underscored topic name, and (2) keeping the
// package and test names identical makes the promotion a pure move, keeps
// `.evolve/evals/continuation-operator-cli.md`'s `-run '^TestC1515_00N'`
// evidence commands valid, and keeps the cycle-1515 provenance readable in CI
// output. Whether the ORIGINAL `go/acs/cycle1515/` copy is deleted or left in
// place as a historical record is deliberately NOT pinned — either is correct,
// Builder documents the call in build-report.md.
//
// Predicate strategy (the cycle-85 degenerate-predicate ban): the load-bearing
// assertions here all drive the real Go toolchain against the real repository —
// `go test` actually EXECUTES the promoted predicates (which in turn build the
// `evolve` binary and drive the production inboxmover/continuation seams), and
// `go list` reports the real package/dependency graph. Adding text to a source
// file cannot satisfy any of 001-004. Predicate 005 is an inherent
// config-presence check on the CI workflow and carries the explicit waiver.
//
// Reliability (flaky-predicate-shape rules): every subprocess names exactly ONE
// package (never a `/...` sweep of test execution — the single `go list`
// pattern-expansion in 002 is a metadata query, not a test run), every
// invocation sets an explicit cmd.Dir or `git -C`, there is no wall-clock
// deadline, no literal PID and no un-reaped load generator.
package cycle1518

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// promotedPkgRel is the durable destination, relative to the go module root.
const promotedPkgRel = "./acs/regression/cycle1515"

// promotedPkgDir is the same destination as a repo-relative directory.
const promotedPkgDir = "go/acs/regression/cycle1515"

// promotedTests are the eight cycle-1515 predicates that must survive the
// promotion by name: 001-002 pin `registry-release-on-park-consume` (park
// releases the binding, preserves the pointer, invents no annotation),
// 003 pins the live-item guard, 004-008 pin the `continuation-operator-cli`
// surface. Losing any one of them silently narrows the durable gate.
var promotedTests = []string{
	"TestC1515_001_ParkReleasesBindingAndPreservesPointer",
	"TestC1515_002_ParkOfUnboundItemInventsNoAnnotation",
	"TestC1515_003_ScopeResolveRefusesRetiredBinding",
	"TestC1515_004_ContinuationListShowsBindings",
	"TestC1515_005_ContinuationListOnEmptyRegistryIsCleanExit",
	"TestC1515_006_ContinuationReleaseReleasesAndAnnotates",
	"TestC1515_007_ContinuationReleaseRejectsUnknownScope",
	"TestC1515_008_ContinuationRejectsMalformedInvocations",
}

// productionPaths are the trees this promotion must NOT touch: it is a
// test-only move.
var productionPaths = []string{
	"go/cmd/evolve/",
	"go/internal/continuation/",
	"go/internal/inboxmover/",
}

// moduleRoot returns <worktree>/go by walking up from this package's directory,
// so no predicate depends on process cwd (fleet lanes and the main tree differ).
func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd() // go test runs in the package dir: <root>/go/acs/cycle1518
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, serr := os.Stat(filepath.Join(dir, "go.mod")); serr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("no go.mod found walking up from %s", wd)
	return ""
}

// repoRoot returns the worktree root (parent of the go module root).
func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Dir(moduleRoot(t))
}

// runIn executes name with args rooted AT dir (explicit cmd.Dir — never process
// cwd) and returns combined output plus the exit code.
func runIn(t *testing.T, dir, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running %s %v in %s: %v\n%s", name, args, dir, err, buf.String())
	}
	return buf.String(), code
}

// ---------------------------------------------------------------------------
// Task: promote-cycle1515-continuation-predicates-to-acs-regression
// ---------------------------------------------------------------------------

// TestC1518_001_PromotedPredicatesRunGreenOnTheDurablePath EXECUTES the promoted
// package exactly the way the acs-durable CI job would reach it, and requires
// every one of the eight cycle-1515 predicates to report PASS. Running the suite
// is what makes this a behavioural predicate: the promoted predicates themselves
// build the `evolve` binary and drive the real inboxmover/continuation seams, so
// a copied-but-broken file, a file whose build tag excludes it, or a package that
// silently dropped predicates all fail here.
//
// The explicit PASS-name check is load-bearing anti-vacuity: `go test` on a
// package containing ZERO tests exits 0, so exit code alone would be satisfied
// by an empty stub file at the destination.
func TestC1518_001_PromotedPredicatesRunGreenOnTheDurablePath(t *testing.T) {
	goRoot := moduleRoot(t)

	out, code := runIn(t, goRoot, "go", "test", "-count=1", "-tags", "acs", "-v", promotedPkgRel)
	if code != 0 {
		t.Fatalf("RED: `go test -tags acs %s` exited %d — the cycle-1515 continuation predicates are not present-and-green on the durable path, so `acs-durable` CI still gives the shipped continuation CLI and park/consume release binding ZERO standing protection.\n%s",
			promotedPkgRel, code, out)
	}
	for _, name := range promotedTests {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not report `--- PASS: %s` — that predicate was lost or renamed in the promotion, silently narrowing the durable gate.\n%s",
				promotedPkgRel, name, out)
		}
	}
	if strings.Contains(out, "--- SKIP: TestC1515_") {
		t.Errorf("RED: a promoted cycle-1515 predicate SKIPPED on the durable path — a gate that skips is not a gate.\n%s", out)
	}
}

// TestC1518_002_PromotedPackageIsSweptByTheDurableGateGlob asks the Go toolchain
// where the promoted package actually lives on disk and requires that directory
// to sit inside `<module>/acs/regression/`, which is precisely the subtree the
// acs-durable CI step's `./acs/regression/...` pattern expands to. This is the
// durability criterion itself: a file that is green when named directly but does
// not fall inside the swept subtree closes nothing. Asking the toolchain for the
// resolved package directory — rather than reading the workflow's text or
// expanding the whole subtree (a contention-sensitive sweep under fleet load) —
// is what makes this behavioural AND reliable.
func TestC1518_002_PromotedPackageIsSweptByTheDurableGateGlob(t *testing.T) {
	goRoot := moduleRoot(t)

	out, code := runIn(t, goRoot, "go", "list", "-tags", "acs", "-f", "{{.Dir}}", promotedPkgRel)
	if code != 0 {
		t.Fatalf("RED: `go list -tags acs %s` exited %d — the cycle-1515 predicates do not resolve as a package under the durable regression tree; they are still per-cycle-only (go/acs/cycle1515), which CI never sweeps.\n%s",
			promotedPkgRel, code, out)
	}
	dir := strings.TrimSpace(out)
	sweptRoot := filepath.Join(goRoot, "acs", "regression") + string(filepath.Separator)
	if !strings.HasPrefix(dir, sweptRoot) {
		t.Errorf("RED: the promoted package resolves to %q, which is OUTSIDE %q — the acs-durable job's `./acs/regression/...` pattern would not reach it, so the promotion protects nothing.", dir, sweptRoot)
	}
	if filepath.Base(dir) != "cycle1515" {
		t.Errorf("RED: promoted package directory is %q, expected basename %q — the pinned durable destination is %s.", dir, "cycle1515", promotedPkgDir)
	}
}

// TestC1518_003_PromotedPredicatesStillDriveProductionSeams is the anti-stub
// control. Predicates 001/002 are satisfiable in principle by a destination file
// that merely declares eight trivially-passing test functions with the right
// names; this one asks the toolchain for the promoted package's REAL test
// dependency graph and requires it to still reach the production packages the
// pins exist to protect, then re-runs the single negative predicate (007, the
// unknown-scope rejection) and requires it to genuinely execute and pass.
func TestC1518_003_PromotedPredicatesStillDriveProductionSeams(t *testing.T) {
	goRoot := moduleRoot(t)

	out, code := runIn(t, goRoot, "go", "list", "-tags", "acs",
		"-f", "{{join .TestImports \"\\n\"}}", promotedPkgRel)
	if code != 0 {
		t.Fatalf("RED: `go list -tags acs -f TestImports %s` exited %d — the promoted package does not resolve.\n%s", promotedPkgRel, code, out)
	}
	for _, want := range []string{
		"github.com/mickeyyaya/evolve-loop/go/internal/continuation",
		"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RED: the promoted package's test imports do not include %q — the promotion kept the test NAMES but lost the production seam they drive, so the durable gate protects nothing.\nTestImports:\n%s", want, out)
		}
	}

	const negative = "TestC1515_007_ContinuationReleaseRejectsUnknownScope"
	runOut, runCode := runIn(t, goRoot, "go", "test", "-count=1", "-tags", "acs", "-v",
		"-run", "^"+negative+"$", promotedPkgRel)
	if runCode != 0 {
		t.Fatalf("RED: the promoted negative predicate %s exited %d — the anti-no-op pin (a typo'd scope id must not read as a successful release) does not survive on the durable path.\n%s", negative, runCode, runOut)
	}
	if !strings.Contains(runOut, "--- PASS: "+negative) {
		t.Errorf("RED: `-run ^%s$` on %s ran NO such test — the negative predicate was dropped in the promotion.\n%s", negative, promotedPkgRel, runOut)
	}
}

// TestC1518_004_PromotionTouchesNoProductionCode is the anti-overreach control
// and a regression pin on the shipped behaviour itself. The task is a test-only
// move: `registry-release-on-park-consume` and `continuation-operator-cli` are
// already live and green, so any edit under cmd/evolve, internal/continuation or
// internal/inboxmover in this cycle's diff is scope creep against working code —
// the exact shape that turns a durability chore into a regression.
func TestC1518_004_PromotionTouchesNoProductionCode(t *testing.T) {
	root := repoRoot(t)

	out, code := runIn(t, root, "git", "-C", root, "status", "--porcelain")
	if code != 0 {
		t.Fatalf("`git -C %s status --porcelain` exited %d:\n%s", root, code, out)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Porcelain v1: "XY <path>" (rename shows "old -> new"); take the tail.
		fields := strings.Fields(line)
		path := fields[len(fields)-1]
		for _, prod := range productionPaths {
			if strings.HasPrefix(path, prod) {
				t.Errorf("RED: this cycle's working tree modifies production path %q (%s) — the promotion is test-only; touching the already-shipped continuation code risks regressing the very behaviour the predicates pin.", path, line)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, promotedPkgDir)); err != nil {
		t.Errorf("RED: destination %s does not exist (%v) — nothing was promoted.", promotedPkgDir, err)
	}
}

// TestC1518_005_DurableGateStillSweepsTheRegressionTree pins the CI side of the
// durability contract: the promotion is only durable because the `acs-durable`
// job runs the recursive regression pattern. If that step were ever narrowed to
// an explicit package list, this promotion would stop protecting anything.
//
// acs-predicate: config-check — this is an inherent configuration-presence
// assertion on a GitHub Actions workflow, which cannot be executed locally; the
// behavioural half of the same contract is predicate 002, which proves the
// pattern really expands to include the promoted package.
func TestC1518_005_DurableGateStillSweepsTheRegressionTree(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".github", "workflows", "ci.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	const want = "go test -count=1 -tags acs ./acs/regression/..."
	if !strings.Contains(string(raw), want) {
		t.Errorf("RED: %s no longer runs %q — the durable gate's recursive sweep is what makes promoting a predicate into acs/regression/ meaningful; without it the promotion protects nothing.", path, want)
	}
	if _, serr := os.Stat(filepath.Join(root, ".github")); serr != nil {
		t.Fatalf("no .github tree at %s: %v", root, serr)
	}
}
