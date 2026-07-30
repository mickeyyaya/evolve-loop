package evalqualitycheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// flakylint_test.go — authoring-time flaky-shape lint over ACS predicate
// sources (acs-metapredicate-suite-scope). Each flagged fixture is the shape
// that burned cycles 1173/1175/1178 (whole-package meta-predicates whose inner
// tests are environment-sensitive under fleet load); each clean fixture is the
// sanctioned equivalent that must NOT be flagged.

// writePredicateDir writes src as predicates_test.go into a fresh temp dir
// (dir mode — the shape `evolve eval quality-check -predicates` passes).
func writePredicateDir(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// lintDir runs the lint over src-in-a-temp-dir and returns the findings, failing
// the test on error. The file receipt is asserted for EVERY fixture: a lint that
// parsed nothing must never be able to look like a clean tree.
func lintDir(t *testing.T, src string) []FlakyFinding {
	t.Helper()
	var report FlakyLintReport
	report, err := LintFlakyPredicates(writePredicateDir(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if report.Linted() != 1 || len(report.Files) != 1 || report.Files[0] != "predicates_test.go" {
		t.Fatalf("receipt must name the one file parsed; got Linted=%d Files=%v", report.Linted(), report.Files)
	}
	return report.Findings
}

// findingFor returns the first finding for fn whose reason contains substr.
func findingFor(fs []FlakyFinding, fn, substr string) (FlakyFinding, bool) {
	for _, f := range fs {
		if f.Func == fn && strings.Contains(f.Reason, substr) {
			return f, true
		}
	}
	return FlakyFinding{}, false
}

// --- pattern 1: suite-scope go-test shells ----------------------------------

const suiteScopeSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

const corePkg = "./internal/core"

func TestC9999_WholeModuleSweep(t *testing.T) {
	if err := exec.Command("go", "test", "./...").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_RecursiveBridge(t *testing.T) {
	if err := exec.Command("go", "test", "./internal/bridge/...").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_KnownSlowCore(t *testing.T) {
	if err := exec.Command("go", "test", corePkg).Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_KnownSlowCmdEvolve(t *testing.T) {
	if err := exec.Command("go", "test", "./cmd/evolve").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_MultiPackage(t *testing.T) {
	if err := exec.Command("go", "test", "./internal/evalgate", "./internal/fleet").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_ShellRecursive(t *testing.T) {
	if err := exec.Command("bash", "-c", "go test -count=1 ./internal/core/... && echo ok").Run(); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_SuiteScope_Flagged — `./...`, `/...` expansion, the known
// 40s+ suites (internal/core, cmd/evolve — including via const indirection,
// the cycle-1117 bridgePkg shape), multi-package invocations, and sh -c
// wrapped forms are all suite-scope findings of the concurrency class.
func TestLintFlaky_SuiteScope_Flagged(t *testing.T) {
	fs := lintDir(t, suiteScopeSrc)
	cases := []struct{ fn, substr string }{
		{"TestC9999_WholeModuleSweep", "./..."},
		{"TestC9999_RecursiveBridge", "./internal/bridge/..."},
		{"TestC9999_KnownSlowCore", "internal/core"},
		{"TestC9999_KnownSlowCmdEvolve", "cmd/evolve"},
		{"TestC9999_MultiPackage", "multi-package"},
		{"TestC9999_ShellRecursive", "./internal/core/..."},
	}
	for _, c := range cases {
		f, ok := findingFor(fs, c.fn, c.substr)
		if !ok {
			t.Errorf("%s: no suite-scope finding containing %q; got %+v", c.fn, c.substr, fs)
			continue
		}
		if f.Class != FlakyClassConcurrency {
			t.Errorf("%s: Class=%q, want %q", c.fn, f.Class, FlakyClassConcurrency)
		}
		if f.File != "predicates_test.go" {
			t.Errorf("%s: File=%q, want predicates_test.go", c.fn, f.File)
		}
	}
}

// --- pattern 2: wall-clock deadlines ----------------------------------------

const wallClockSrc = `//go:build acs

package cycle9999

import (
	"testing"
	"time"
)

func TestC9999_PollUntilDeadline(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
}

func TestC9999_ElapsedWallClock(t *testing.T) {
	start := time.Now()
	if time.Since(start) > 2*time.Second {
		t.Fatal("too slow")
	}
}
`

// TestLintFlaky_WallClockDeadline_Flagged — time.Now()-derived deadlines
// (.Add/.Before chains) and time.Since elapsed checks are async-wait class.
func TestLintFlaky_WallClockDeadline_Flagged(t *testing.T) {
	fs := lintDir(t, wallClockSrc)
	cases := []struct{ fn, substr string }{
		{"TestC9999_PollUntilDeadline", "time.Now().Add"},
		{"TestC9999_PollUntilDeadline", "time.Now().Before"},
		{"TestC9999_ElapsedWallClock", "time.Since"},
	}
	for _, c := range cases {
		f, ok := findingFor(fs, c.fn, c.substr)
		if !ok {
			t.Errorf("%s: no wall-clock finding containing %q; got %+v", c.fn, c.substr, fs)
			continue
		}
		if f.Class != FlakyClassAsyncWait {
			t.Errorf("%s: Class=%q, want %q", c.fn, f.Class, FlakyClassAsyncWait)
		}
	}
}

// --- pattern 3: hardcoded PIDs ----------------------------------------------

const pidSrc = `//go:build acs

package cycle9999

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func TestC9999_HardcodedKill(t *testing.T) {
	if err := syscall.Kill(4242, 0); err != nil {
		t.Fatal("driver process not alive")
	}
}

func TestC9999_HardcodedFindProcess(t *testing.T) {
	if _, err := os.FindProcess(12345); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_HardcodedProcPath(t *testing.T) {
	if _, err := os.Stat("/proc/4242/status"); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_KillDashZero(t *testing.T) {
	if err := exec.Command("kill", "-0", "4242").Run(); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_HardcodedPID_Flagged — literal PIDs below 100000 used for
// liveness (syscall.Kill / os.FindProcess / kill -0 / /proc paths) are
// environment class: PIDs are never stable across hosts or runs.
func TestLintFlaky_HardcodedPID_Flagged(t *testing.T) {
	fs := lintDir(t, pidSrc)
	cases := []struct{ fn, substr string }{
		{"TestC9999_HardcodedKill", "4242"},
		{"TestC9999_HardcodedFindProcess", "12345"},
		{"TestC9999_HardcodedProcPath", "4242"},
		{"TestC9999_KillDashZero", "4242"},
	}
	for _, c := range cases {
		f, ok := findingFor(fs, c.fn, c.substr)
		if !ok {
			t.Errorf("%s: no hardcoded-PID finding containing %q; got %+v", c.fn, c.substr, fs)
			continue
		}
		if f.Class != FlakyClassEnvironment {
			t.Errorf("%s: Class=%q, want %q", c.fn, f.Class, FlakyClassEnvironment)
		}
	}
}

// --- pattern 4: git without -C ----------------------------------------------

const gitNoCSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

func TestC9999_GitCwdDependent(t *testing.T) {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil || len(out) != 0 {
		t.Fatal("dirty")
	}
}

func TestC9999_ShellGitCwdDependent(t *testing.T) {
	if err := exec.Command("bash", "-c", "git log --oneline | head -1").Run(); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_GitWithoutC_Flagged — subprocess git resolving the repo from
// process cwd (no -C, no cmd.Dir) is environment class, in both direct-exec
// and sh -c forms.
func TestLintFlaky_GitWithoutC_Flagged(t *testing.T) {
	fs := lintDir(t, gitNoCSrc)
	for _, fn := range []string{"TestC9999_GitCwdDependent", "TestC9999_ShellGitCwdDependent"} {
		f, ok := findingFor(fs, fn, "-C")
		if !ok {
			t.Errorf("%s: no git-without--C finding; got %+v", fn, fs)
			continue
		}
		if f.Class != FlakyClassEnvironment {
			t.Errorf("%s: Class=%q, want %q", fn, f.Class, FlakyClassEnvironment)
		}
	}
}

const gitAnchoredSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

func TestC9999_GitDashC(t *testing.T) {
	if err := exec.Command("git", "-C", "/repo", "status", "--porcelain").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_GitCmdDir(t *testing.T) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = "/repo"
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_ShellGitAfterCd(t *testing.T) {
	if err := exec.Command("bash", "-c", "cd /repo && git status --porcelain").Run(); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_GitAnchored_Clean — git WITH -C, git with cmd.Dir set, and
// sh -c scripts that cd first are all repo-anchored: no findings.
func TestLintFlaky_GitAnchored_Clean(t *testing.T) {
	fs := lintDir(t, gitAnchoredSrc)
	if len(fs) != 0 {
		t.Errorf("anchored git fixtures must not be flagged; got %+v", fs)
	}
}

const gitAppendIdiomSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
`

// TestLintFlaky_GitDashCInAppend_Clean — the corpus house idiom (cycles
// 962/968): -C passed inside an append([]string{"-C", dir}, args...) composite
// is anchored; the lint must find the literal at any nesting depth.
func TestLintFlaky_GitDashCInAppend_Clean(t *testing.T) {
	fs := lintDir(t, gitAppendIdiomSrc)
	if len(fs) != 0 {
		t.Errorf("append-composed -C must count as anchored; got %+v", fs)
	}
}

// --- pattern 5: unreaped load-generation ------------------------------------

const loadGenSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

func TestC9999_UnreapedYes(t *testing.T) {
	cmd := exec.Command("yes")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_UnreapedBusyLoop(t *testing.T) {
	if err := exec.Command("sh", "-c", "while true; do :; done &").Start(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_UnreapedStress(t *testing.T) {
	if err := exec.Command("bash", "-c", "stress --cpu 4").Start(); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_LoadGenUnreaped_Flagged — exec.Command (no context) spawning
// load generators (`yes`, `stress`, shell busy loops) is resource-leak class:
// nothing reaps the child when the predicate exits.
func TestLintFlaky_LoadGenUnreaped_Flagged(t *testing.T) {
	fs := lintDir(t, loadGenSrc)
	cases := []struct{ fn, substr string }{
		{"TestC9999_UnreapedYes", "yes"},
		{"TestC9999_UnreapedBusyLoop", "busy loop"},
		{"TestC9999_UnreapedStress", "stress"},
	}
	for _, c := range cases {
		f, ok := findingFor(fs, c.fn, c.substr)
		if !ok {
			t.Errorf("%s: no load-gen finding containing %q; got %+v", c.fn, c.substr, fs)
			continue
		}
		if f.Class != FlakyClassResourceLeak {
			t.Errorf("%s: Class=%q, want %q", c.fn, f.Class, FlakyClassResourceLeak)
		}
	}
}

const goBuildVetSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

func TestC9999_BinaryBuilds(t *testing.T) {
	if out, err := exec.Command("go", "build", "-C", "/repo/go", "-o", "/tmp/evolve", "./cmd/evolve").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
}

func TestC9999_VetClean(t *testing.T) {
	if out, err := exec.Command("go", "vet", "./internal/core").CombinedOutput(); err != nil {
		t.Fatalf("vet: %v\n%s", err, out)
	}
}
`

// TestLintFlaky_GoBuildVet_NotSuiteScope — the suite-scope rule targets
// go-TEST shells: a direct `go build` / `go vet` over the same patterns is a
// compile, not a 40s+ test suite under contention (cycle-969 buildEvolve /
// cycle-941 module_builds corpus shapes must stay clean).
func TestLintFlaky_GoBuildVet_NotSuiteScope(t *testing.T) {
	fs := lintDir(t, goBuildVetSrc)
	if len(fs) != 0 {
		t.Errorf("go build/vet invocations must not be suite-scope findings; got %+v", fs)
	}
}

// --- clean composite ---------------------------------------------------------

const cleanSrc = `//go:build acs

package cycle9999

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

const carryoverBaseline = 135

func TestC9999_SingleTouchedPackageGreen(t *testing.T) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./internal/evalgate")
	if err := cmd.Run(); err != nil {
		t.Fatalf("go test: %v", err)
	}
}

func TestC9999_LoadGenReaped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := exec.CommandContext(ctx, "yes").Start(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_RuntimePIDLiveness(t *testing.T) {
	pid := os.Getpid()
	if _, err := os.FindProcess(pid); err != nil {
		t.Fatal(err)
	}
	if carryoverBaseline != 135 {
		t.Fatal("baseline drifted")
	}
}
`

// TestLintFlaky_CleanEquivalents_NotFlagged — the sanctioned shapes: a single
// named-package go test, CommandContext-reaped load-gen, runtime-discovered
// PIDs, and plain small int consts outside liveness contexts.
func TestLintFlaky_CleanEquivalents_NotFlagged(t *testing.T) {
	fs := lintDir(t, cleanSrc)
	if len(fs) != 0 {
		t.Errorf("clean fixtures must not be flagged; got %+v", fs)
	}
}

// --- modes + error paths -----------------------------------------------------

// TestLintFlaky_SingleFileMode — a direct predicates_test.go path (not a dir)
// lints identically.
func TestLintFlaky_SingleFileMode(t *testing.T) {
	dir := writePredicateDir(t, wallClockSrc)
	path := filepath.Join(dir, "predicates_test.go")
	report, err := LintFlakyPredicates(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findingFor(report.Findings, "TestC9999_PollUntilDeadline", "time.Now().Add"); !ok {
		t.Errorf("file mode: expected the deadline finding; got %+v", report.Findings)
	}
	if report.Path != path || report.Linted() != 1 {
		t.Errorf("file-mode receipt: Path=%q Linted=%d, want %q and 1", report.Path, report.Linted(), path)
	}
}

// TestLintFlaky_MissingPath_Error — a bad path surfaces as error (the CLI
// degrades it to a loud advisory skip, never a block).
func TestLintFlaky_MissingPath_Error(t *testing.T) {
	if _, err := LintFlakyPredicates("/no/such/predicates_test.go"); err == nil {
		t.Error("missing path: want error")
	}
}

// TestLintFlaky_UnparseableSource_Error — garbage source is an error, not a
// silent zero-findings pass (fail-open must be loud at the caller).
func TestLintFlaky_UnparseableSource_Error(t *testing.T) {
	dir := writePredicateDir(t, "package cycle9999\nfunc {{{")
	if _, err := LintFlakyPredicates(dir); err == nil {
		t.Error("unparseable source: want error")
	}
}

// TestLintFlaky_DirWithNoGoFiles_Error is the H2 silent-clean pin: a directory
// holding no .go files linted NOTHING. Returning (nil, nil) there made
// "0 findings over 0 files" byte-indistinguishable from "0 findings over a
// linted tree" — the same class as the dropped -predicates flag. It must be an
// error, and the report must still name the path it was asked about.
func TestLintFlaky_DirWithNoGoFiles_Error(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("no go here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := LintFlakyPredicates(dir)
	if err == nil {
		t.Fatal("a dir with zero .go files must ERROR, never read as a clean lint")
	}
	if !strings.Contains(err.Error(), "not a clean result") {
		t.Errorf("error %q should say outright that nothing was linted", err)
	}
	if report.Linted() != 0 || report.Path != dir {
		t.Errorf("report on error: Path=%q Linted=%d, want %q and 0", report.Path, report.Linted(), dir)
	}
}

// --- M4: argv-position awareness (the dominant false-positive class) ---------

const nonTestExecPatternSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

func TestC9999_RipgrepOverCore(t *testing.T) {
	if err := exec.Command("rg", "--files", "./internal/core").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_VetRecursive(t *testing.T) {
	if err := exec.Command("go", "vet", "./internal/bridge/...").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_ListRecursive(t *testing.T) {
	if err := exec.Command("go", "list", "./...").Run(); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_PatternInNonTestExecArgv_NotFlagged is the M4 pin: a package
// pattern handed to a subprocess that is NOT `go test` — rg, go vet, go list — is
// a search or a compile, never a 40s+ suite under contention. Flagging it prints
// a claim that is false about the code, which is how authors learn to ignore a
// linter. This shape was the single largest false-positive source in the
// 121-of-284 corpus calibration.
func TestLintFlaky_PatternInNonTestExecArgv_NotFlagged(t *testing.T) {
	fs := lintDir(t, nonTestExecPatternSrc)
	if len(fs) != 0 {
		t.Errorf("patterns in non-go-test exec argvs must not be suite-scope findings; got %+v", fs)
	}
}

const mixedExecPatternSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

func TestC9999_VetThenTestSamePattern(t *testing.T) {
	if err := exec.Command("go", "vet", "./internal/core").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("go", "test", "./internal/core").Run(); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_GoTestArgvWins_EvenBesideNonTestUse — a same-function `go vet` on
// the pattern must NOT suppress the `go test` on it. The old design's benign-set
// let one compile hide a real suite shell; the argv-position index gives go-test
// evidence priority.
func TestLintFlaky_GoTestArgvWins_EvenBesideNonTestUse(t *testing.T) {
	fs := lintDir(t, mixedExecPatternSrc)
	if _, ok := findingFor(fs, "TestC9999_VetThenTestSamePattern", "internal/core"); !ok {
		t.Errorf("a go-test argv must still flag even beside a go-vet use of the same pattern; got %+v", fs)
	}
}

const helperMediatedPatternSrc = `//go:build acs

package cycle9999

import "testing"

func TestC9999_HelperShellsSuite(t *testing.T) {
	runSuite(t, "./internal/core/...")
}

func runSuite(t *testing.T, pkg string) { t.Helper() }
`

// TestLintFlaky_HelperMediatedPattern_StillFlagged — a pattern that reaches NO
// exec argv is unresolvable here: it may be handed to a helper that shells
// `go test` (the shape above). The advisory note stands — the safe direction for
// an advisory lint is to over-report the unknowable, not to go quiet.
func TestLintFlaky_HelperMediatedPattern_StillFlagged(t *testing.T) {
	fs := lintDir(t, helperMediatedPatternSrc)
	if _, ok := findingFor(fs, "TestC9999_HelperShellsSuite", "./internal/core/..."); !ok {
		t.Errorf("a pattern in no exec argv must stay flagged (helper-mediated, unresolvable); got %+v", fs)
	}
}

const acsassertHelperSrc = `//go:build acs

package cycle9999

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC9999_GoVetCleanViaHelper(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	if _, _, code, err := acsassert.SubprocessOutput("go", "vet", "-C", goDir, "./..."); err != nil || code != 0 {
		t.Fatal("vet")
	}
}

func TestC9999_GoTestSweepViaHelper(t *testing.T) {
	if _, _, code, _ := acsassert.SubprocessOutput("go", "test", "./internal/core"); code != 0 {
		t.Fatal("test")
	}
}

func TestC9999_LoadGenViaHelper(t *testing.T) {
	if _, _, _, err := acsassert.SubprocessOutput("yes"); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_GitViaHelper(t *testing.T) {
	if _, _, _, err := acsassert.SubprocessOutput("git", "status", "--porcelain"); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_AcsassertSubprocessOutputIsAnExecConstructor — the corpus's real
// exec constructor is acsassert.SubprocessOutput (198 of 282 historical acs dirs
// use it; 20 use exec.Command). Recognizing only exec.Command made the M4
// argv-position rule technically implemented but practically INERT: a
// `go vet ./...` through the house helper reached no recognized exec argv, landed
// in the "unresolvable" bucket, and kept its false suite-scope finding — which was
// the single largest remaining false-positive class on the real corpus.
//
// All four consequences are asserted together, because they are one decision:
// the helper's go-vet sweep is clean, its go-test on a known-slow package still
// fires, and — since it binds no context — its load-gen and its bare git are
// correctly flagged.
func TestLintFlaky_AcsassertSubprocessOutputIsAnExecConstructor(t *testing.T) {
	fs := lintDir(t, acsassertHelperSrc)

	if f, ok := findingFor(fs, "TestC9999_GoVetCleanViaHelper", "suite-scope"); ok {
		t.Errorf("go vet through the house helper must not be a suite-scope finding; got %+v", f)
	}
	if _, ok := findingFor(fs, "TestC9999_GoTestSweepViaHelper", "internal/core"); !ok {
		t.Errorf("a go TEST through the house helper must still flag the known-slow suite; got %+v", fs)
	}
	if f, ok := findingFor(fs, "TestC9999_LoadGenViaHelper", "unreaped load-generation"); !ok {
		t.Errorf("the house helper binds no context, so its load-gen is un-reaped; got %+v", fs)
	} else if f.Class != FlakyClassResourceLeak {
		t.Errorf("load-gen Class=%q, want %q", f.Class, FlakyClassResourceLeak)
	}
	if _, ok := findingFor(fs, "TestC9999_GitViaHelper", "-C"); !ok {
		t.Errorf("git through the house helper is still cwd-dependent; got %+v", fs)
	}
}

// helperHopSrc is the CANONICAL corpus idiom, verbatim in shape from
// go/acs/cycle1034: the package pattern lives in a const the TEST function names,
// and the -run narrowing lives in a same-file helper that takes the package as a
// parameter.
const helperHopSrc = `//go:build acs

package cycle9999

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	if _, _, code, _ := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg); code != 0 {
		t.Fatal("suite red")
	}
}

func TestC9999_001_NarrowedThroughHelper(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_PreClassBuckets")
}
`

// TestLintFlaky_HelperHopResolvesNarrowedInvocation is the HIGH-2 pin. Indexing
// only the test function's OWN body saw no exec argv here, fell into the
// "unresolvable → keep the note" branch, and printed "narrow the invocation with
// -run" at code that already narrows — 142 of 297 corpus findings (48%), measured
// on the live 282-dir corpus. Following ONE level into the same-package helper,
// with the call's args bound to the helper's params, resolves it correctly.
func TestLintFlaky_HelperHopResolvesNarrowedInvocation(t *testing.T) {
	fs := lintDir(t, helperHopSrc)
	if len(fs) != 0 {
		t.Errorf("a -run-narrowed invocation reached through a same-file helper must not be flagged; got %+v", fs)
	}
}

const helperHopWideSrc = `//go:build acs

package cycle9999

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func assertSuitePasses(t *testing.T, pkg string) {
	t.Helper()
	if _, _, code, _ := acsassert.SubprocessOutput("go", "test", "-count=1", pkg); code != 0 {
		t.Fatal("suite red")
	}
}

func TestC9999_001_WideThroughHelper(t *testing.T) {
	assertSuitePasses(t, corePkg)
}
`

// TestLintFlaky_HelperHopStillFlagsWideInvocation — the helper hop must resolve
// BOTH ways. A helper that shells the whole known-slow package with no -run is
// the real cycles-1173/1175/1178 shape and must still fire; a hop that only ever
// suppressed would have traded 48% false positives for false negatives.
func TestLintFlaky_HelperHopStillFlagsWideInvocation(t *testing.T) {
	fs := lintDir(t, helperHopWideSrc)
	if _, ok := findingFor(fs, "TestC9999_001_WideThroughHelper", "internal/core"); !ok {
		t.Errorf("an un-narrowed known-slow suite reached through a helper must still flag; got %+v", fs)
	}
}

const helperHopTwoLevelSrc = `//go:build acs

package cycle9999

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func inner(t *testing.T, pkg string) {
	if _, _, code, _ := acsassert.SubprocessOutput("go", "test", "-run", "TestX", pkg); code != 0 {
		t.Fatal("red")
	}
}

func outer(t *testing.T, pkg string) { inner(t, pkg) }

func TestC9999_001_TwoLevelsDown(t *testing.T) {
	outer(t, corePkg)
}
`

// TestLintFlaky_HelperHopIsDepthOneOnly pins the bound: the hop follows exactly
// one level, so a two-level chain stays unresolvable and KEEPS its advisory note.
// Depth 1 is what makes the walk terminate without a visited set; the residual is
// documented rather than silently deepened.
func TestLintFlaky_HelperHopIsDepthOneOnly(t *testing.T) {
	fs := lintDir(t, helperHopTwoLevelSrc)
	if _, ok := findingFor(fs, "TestC9999_001_TwoLevelsDown", "internal/core"); !ok {
		t.Errorf("two levels down is unresolvable and must keep its note (safe direction); got %+v", fs)
	}
}

// --- M6: -run narrowing suppresses the known-slow finding --------------------

const runNarrowedSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

func TestC9999_NarrowedKnownSlow(t *testing.T) {
	if err := exec.Command("go", "test", "-run", "TestOneThing", "./internal/core").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_NarrowedKnownSlowEqualsForm(t *testing.T) {
	if err := exec.Command("go", "test", "-run=TestOneThing", "./cmd/evolve").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC9999_NarrowedShellForm(t *testing.T) {
	if err := exec.Command("bash", "-c", "go test -run TestOneThing ./internal/core").Run(); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_RunNarrowedKnownSlow_NotFlagged is the M6 pin: the known-slow
// finding estimates the cost of running a whole 40s+ package. A `-run`-narrowed
// invocation runs a handful of tests, so the estimate no longer holds and the
// finding would be a false claim — in the direct, `-run=` and sh -c forms alike.
func TestLintFlaky_RunNarrowedKnownSlow_NotFlagged(t *testing.T) {
	fs := lintDir(t, runNarrowedSrc)
	if len(fs) != 0 {
		t.Errorf("-run-narrowed known-slow invocations must not be flagged; got %+v", fs)
	}
}

const runNarrowedRecursiveSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

func TestC9999_NarrowedButRecursive(t *testing.T) {
	if err := exec.Command("go", "test", "-run", "TestOneThing", "./internal/bridge/...").Run(); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_RunNarrowedRecursive_StillFlagged — the M6 boundary: -run selects
// which TESTS run, not which PACKAGES are built and loaded, so a narrowed
// recursive sweep still pays whole-subtree cost and stays flagged.
func TestLintFlaky_RunNarrowedRecursive_StillFlagged(t *testing.T) {
	fs := lintDir(t, runNarrowedRecursiveSrc)
	if _, ok := findingFor(fs, "TestC9999_NarrowedButRecursive", "./internal/bridge/..."); !ok {
		t.Errorf("-run does not bound a recursive sweep; it must stay flagged. got %+v", fs)
	}
}

const runNarrowedAndWideSrc = `//go:build acs

package cycle9999

import (
	"os/exec"
	"testing"
)

func TestC9999_NarrowedThenWide(t *testing.T) {
	if err := exec.Command("go", "test", "-run", "TestOneThing", "./internal/core").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("go", "test", "./internal/core").Run(); err != nil {
		t.Fatal(err)
	}
}
`

// TestLintFlaky_NarrowedPlusWideSameSuite_StillFlagged — narrowing is credited
// only when EVERY go-test argv carrying the pattern is narrowed. One wide
// invocation elsewhere in the same function keeps the finding (safe direction).
func TestLintFlaky_NarrowedPlusWideSameSuite_StillFlagged(t *testing.T) {
	fs := lintDir(t, runNarrowedAndWideSrc)
	if _, ok := findingFor(fs, "TestC9999_NarrowedThenWide", "internal/core"); !ok {
		t.Errorf("one wide invocation must keep the known-slow finding; got %+v", fs)
	}
}
