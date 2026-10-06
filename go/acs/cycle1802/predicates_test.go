//go:build acs

package cycle1802

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
)

const fixtureHeader = `//go:build acs

package cycle4242

import (
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var _ = exec.Command
var _ = syscall.Kill
var _ = time.Now
var _ = acsassert.RepoRoot
`

const invertedIdiomPredicate = `
func TestC4242_RetiredHelperGone(t *testing.T) {
	if acsassert.FileContains(t, "f.go", "retiredHelper") {
		t.Errorf("f.go still holds retiredHelper")
	}
}
`

const goRunExitCodePredicate = `
func TestC4242_UsageErrorExitsThree(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/tool", "bogus")
	_ = cmd.Run()
	if cmd.ProcessState.ExitCode() != 3 {
		t.Errorf("want exit 3 on a usage error")
	}
}
`

const absenceMessagePredicate = `
func TestC4242_DuplicateHelperRemoved(t *testing.T) {
	if !acsassert.FileContains(t, "f.go", "helper") {
		t.Errorf("f.go still holds the duplicated helper")
	}
}
`

const flakyShapePredicates = `
func TestC4242_WholeModuleSweep(t *testing.T) {
	if err := exec.Command("go", "test", "./...").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestC4242_WallClockDeadline(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
	}
}

func TestC4242_LiteralPid(t *testing.T) {
	if err := syscall.Kill(4242, 0); err != nil {
		t.Fatal(err)
	}
}

func TestC4242_UnreapedLoad(t *testing.T) {
	if err := exec.Command("yes").Start(); err != nil {
		t.Fatal(err)
	}
}
`

const satisfiablePredicate = `
func TestC4242_RetiredFlagGone(t *testing.T) {
	if !acsassert.FileNotContains(t, "f.go", "retiredFlag") {
		t.Errorf("f.go still holds retiredFlag")
	}
}
`

func tddInputWithPredicates(t *testing.T, body string) core.ReviewInput {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-4242")
	acsDir := filepath.Join(root, "wt", "go", "acs", "cycle4242")
	for _, d := range []string{ws, acsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(acsDir, "predicates_test.go"), []byte(fixtureHeader+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: filepath.Join(root, "wt"), ProjectRoot: root}
}

type pipeRead struct {
	bytes []byte
	err   error
}

func reviewWithLog(t *testing.T, stage config.Stage, in core.ReviewInput) (core.ReviewResult, string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	drained := make(chan pipeRead, 1)
	go func() {
		b, rerr := io.ReadAll(r)
		drained <- pipeRead{b, rerr}
	}()
	saved := os.Stderr
	os.Stderr = w
	res := func() core.ReviewResult {
		defer func() { os.Stderr = saved }()
		return evalgate.NewReviewer(stage).Review(context.Background(), in)
	}()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	got := <-drained
	if got.err != nil {
		t.Fatalf("reading the reviewer log: %v", got.err)
	}
	return res, string(got.bytes)
}

func gateLogLine(log, gateName string) string {
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, "] "+gateName+":") {
			return line
		}
	}
	return ""
}

func requireAll(t *testing.T, label, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("RED: %s must contain %q; got:\n%s", label, want, text)
		}
	}
}

func assertProofBlocksAtEnforce(t *testing.T, body string, wants ...string) {
	t.Helper()
	res, log := reviewWithLog(t, config.StageEnforce, tddInputWithPredicates(t, body))
	if res.Approve {
		t.Fatalf("RED: at enforce a tdd deliverable whose predicate is provably unsatisfiable was approved; the unsatisfiable-predicate-shape gate must reject it. log:\n%s", log)
	}
	requireAll(t, "the rejection reason", res.Reason, append([]string{"unsatisfiable"}, wants...)...)
	if strings.Contains(res.Reason, "ADVISORY: never blocks") {
		t.Errorf("RED: a blocking rejection must not call itself advisory: %q", res.Reason)
	}
	requireAll(t, "the unsatisfiable-predicate-shape log line", gateLogLine(log, "unsatisfiable-predicate-shape"), "stage=enforce", "blocking=true")
}

func assertApprovedWithFindingLogged(t *testing.T, stage config.Stage, body, gateName string, wants ...string) {
	t.Helper()
	res, log := reviewWithLog(t, stage, tddInputWithPredicates(t, body))
	if !res.Approve {
		t.Errorf("RED: at %s this predicate package must be approved; got rejected with %q", stage, res.Reason)
	}
	line := gateLogLine(log, gateName)
	requireAll(t, gateName+" log line", line, append([]string{fmt.Sprintf("stage=%s", stage), "blocking=false"}, wants...)...)
}

func TestC1802_001_EnforceRejectsInvertedIdiomWithFindingAndRemedy(t *testing.T) {
	assertProofBlocksAtEnforce(t, invertedIdiomPredicate,
		"TestC4242_RetiredHelperGone", "inverted-idiom", "acsassert.FileNotContains")
}

func TestC1802_002_EnforceRejectsGoRunExitCodeWithFindingAndRemedy(t *testing.T) {
	assertProofBlocksAtEnforce(t, goRunExitCodePredicate,
		"TestC4242_UsageErrorExitsThree", "go-run-exit-code", "exit code 3", "go build")
}

func TestC1802_003_EnforceRejectionNamesTheProofFindingBeyondTheAdvisoryCap(t *testing.T) {
	var body strings.Builder
	for i := 1; i <= 6; i++ {
		body.WriteString(strings.Replace(absenceMessagePredicate, "TestC4242_DuplicateHelperRemoved", fmt.Sprintf("TestC4242_A%d", i), 1))
	}
	body.WriteString(strings.Replace(invertedIdiomPredicate, "TestC4242_RetiredHelperGone", "TestC4242_Z", 1))
	assertProofBlocksAtEnforce(t, body.String(), "TestC4242_Z", "inverted-idiom", "acsassert.FileNotContains")
}

func TestC1802_004_EnforceApprovesAnAbsenceMessageFinding(t *testing.T) {
	assertApprovedWithFindingLogged(t, config.StageEnforce, absenceMessagePredicate,
		"unsatisfiable-predicate-shape", "TestC4242_DuplicateHelperRemoved", "absence-message")
}

func TestC1802_005_EnforceApprovesEveryFlakyShapeFinding(t *testing.T) {
	assertApprovedWithFindingLogged(t, config.StageEnforce, flakyShapePredicates+absenceMessagePredicate,
		"flaky-predicate-shape", "concurrency", "async-wait", "environment", "resource-leak")
}

func TestC1802_006_EnforceApprovesCleanAndAbsentPredicatePackages(t *testing.T) {
	assertApprovedWithFindingLogged(t, config.StageEnforce, satisfiablePredicate,
		"unsatisfiable-predicate-shape", "CLEAN", "0 findings")
	in := tddInputWithPredicates(t, satisfiablePredicate)
	if err := os.RemoveAll(filepath.Join(in.Worktree, "go", "acs")); err != nil {
		t.Fatal(err)
	}
	if res, log := reviewWithLog(t, config.StageEnforce, in); !res.Approve {
		t.Errorf("RED: a tdd deliverable with no cycle predicate package must be approved at enforce; got %q\nlog:\n%s", res.Reason, log)
	}
}

func TestC1802_007_ShadowLogsAndApprovesEveryKind(t *testing.T) {
	kinds := []struct {
		body, gateName string
		wants          []string
	}{
		{invertedIdiomPredicate, "unsatisfiable-predicate-shape", []string{"TestC4242_RetiredHelperGone", "inverted-idiom"}},
		{goRunExitCodePredicate, "unsatisfiable-predicate-shape", []string{"TestC4242_UsageErrorExitsThree", "go-run-exit-code"}},
		{absenceMessagePredicate, "unsatisfiable-predicate-shape", []string{"TestC4242_DuplicateHelperRemoved", "absence-message"}},
		{flakyShapePredicates, "flaky-predicate-shape", []string{"TestC4242_WholeModuleSweep", "concurrency"}},
	}
	for _, k := range kinds {
		assertApprovedWithFindingLogged(t, config.StageShadow, k.body, k.gateName, k.wants...)
	}
}
