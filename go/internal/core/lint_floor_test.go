package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type lintCall struct {
	moduleDir string
	pkgs      []string
}

type fakeLint struct {
	calls    []lintCall
	findings string
	err      error
}

var _ LintFn = (&fakeLint{}).run

func (f *fakeLint) run(_ context.Context, moduleDir string, pkgs []string) (string, error) {
	f.calls = append(f.calls, lintCall{moduleDir, pkgs})
	return f.findings, f.err
}

func lintFloorReview(t *testing.T, f baseRedFixture, lint *fakeLint, files ...string) ReviewResult {
	t.Helper()
	for _, rel := range files {
		writeFile(t, filepath.Join(f.in.Worktree, filepath.FromSlash(rel)), "package p\n\nfunc F() {}\n")
	}
	return BuildHandoffFloor{{Name: "lint", Run: LintFloorChecks(lint.run)}}.Review(context.Background(), f.in)
}

func TestLintFloor_AFindingRefusesTheHandoffAndNamesFileAndLine(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	lint := &fakeLint{findings: "p/a.go:3:2: Error return value of `f.Close` is not checked (errcheck)"}

	res := lintFloorReview(t, f, lint, "go/p/a.go")

	if res.Approve || !res.Retry || !strings.Contains(res.Reason, "p/a.go:3:2") || !strings.Contains(res.Reason, "errcheck") {
		t.Fatalf("Approve=%v Retry=%v reason=%q, want a refusal that names p/a.go:3:2", res.Approve, res.Retry, res.Reason)
	}
	want := []lintCall{{filepath.Join(f.in.Worktree, "go"), []string{"./p"}}}
	if !reflect.DeepEqual(lint.calls, want) {
		t.Fatalf("lint calls = %+v, want %+v", lint.calls, want)
	}
}

func TestLintFloor_ACleanTreePasses(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	lint := &fakeLint{}

	res := lintFloorReview(t, f, lint, "go/p/a.go")

	if !res.Approve || res.Retry || len(lint.calls) != 1 {
		t.Fatalf("Approve=%v Retry=%v calls=%d reason=%q", res.Approve, res.Retry, len(lint.calls), res.Reason)
	}
}

func TestLintFloor_NoChangedGoPackageRunsNoLint(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	lint := &fakeLint{findings: "never read"}

	res := lintFloorReview(t, f, lint, "docs/note.md")

	if !res.Approve || len(lint.calls) != 0 {
		t.Fatalf("Approve=%v calls=%d, want a pass with no lint run", res.Approve, len(lint.calls))
	}
	in := f.in
	in.Worktree = ""
	if got := LintFloorChecks(lint.run)(context.Background(), in); len(got) != 0 || len(lint.calls) != 0 {
		t.Fatalf("no worktree: lines=%v calls=%d", got, len(lint.calls))
	}
}

func TestLintFloor_AMissingToolFailsOpenWithANamedSignal(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	lint := &fakeLint{err: fmt.Errorf("%w: exec: not found", ErrLintToolMissing)}

	res := lintFloorReview(t, f, lint, "go/p/a.go")

	if !res.Approve {
		t.Fatalf("a missing tool must not refuse; reason=%q", res.Reason)
	}
	var skipped *signalcenter.Event
	for _, e := range f.signals.Recent() {
		if e.Code == CodeBuildLintFloorSkipped {
			skipped = &e
		}
	}
	if skipped == nil || !strings.Contains(skipped.Fields["cause"], "not found") || !strings.Contains(f.stderr.String(), "WARN") {
		t.Fatalf("signal=%+v stderr=%q, want BUILD_LINT_FLOOR_SKIPPED with a cause and a WARN", skipped, f.stderr.String())
	}
}

func TestLintFloor_ALockWaitTimeoutIsARetryableRed(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	lint := &fakeLint{err: fmt.Errorf("%w (budget 10m0s)", ErrLintLockWait)}

	res := lintFloorReview(t, f, lint, "go/p/a.go")

	if res.Approve || !res.Retry || !strings.Contains(res.Reason, "retryable") || !strings.Contains(res.Reason, "10m0s") {
		t.Fatalf("Approve=%v Retry=%v reason=%q, want a retryable lock-wait RED", res.Approve, res.Retry, res.Reason)
	}
}

func TestLintFloor_AnyOtherToolErrorFailsOpenWithTheNamedSignal(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	lint := &fakeLint{err: errors.New("golangci-lint failed with exit 3")}

	res := lintFloorReview(t, f, lint, "go/p/a.go")

	signalled := false
	for _, e := range f.signals.Recent() {
		signalled = signalled || e.Code == CodeBuildLintFloorSkipped && strings.Contains(e.Fields["cause"], "exit 3")
	}
	if !res.Approve || !signalled || !strings.Contains(f.stderr.String(), "WARN") {
		t.Fatalf("Approve=%v signalled=%v stderr=%q, want a fail-open with %s and its cause", res.Approve, signalled, f.stderr.String(), CodeBuildLintFloorSkipped)
	}
}

func TestLintFloor_TheChangedPathsMustBeReadable(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	f.in.Worktree = t.TempDir()
	lint := &fakeLint{findings: "x"}

	res := BuildHandoffFloor{{Name: "lint", Run: LintFloorChecks(lint.run)}}.Review(context.Background(), f.in)

	if !res.Approve || len(lint.calls) != 0 || !strings.Contains(f.stderr.String(), "cannot be read") {
		t.Fatalf("Approve=%v calls=%d stderr=%q, want a fail-open WARN and no lint run", res.Approve, len(lint.calls), f.stderr.String())
	}
}

func TestLintFloorJudge_ReturnsTheFindingLinesAndFailsOpenOnAToolFailure(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	writeFile(t, filepath.Join(f.in.Worktree, "go", "p", "a.go"), "package p\n\nfunc F() {}\n")
	finding := &fakeLint{findings: "p/a.go:3:2: unused (unused)"}
	lines, err := lintFloorJudge(finding.run)(context.Background(), f.in)

	if err != nil || len(lines) != 1 || !strings.Contains(lines[0], "p/a.go:3:2") {
		t.Fatalf("lines=%v err=%v", lines, err)
	}
	broken := &fakeLint{err: errors.New("golangci-lint failed with exit 3")}
	if lines, err = lintFloorJudge(broken.run)(context.Background(), f.in); err != nil || len(lines) != 0 {
		t.Fatalf("lines=%v err=%v, want a fail-open with no lines", lines, err)
	}
}

func TestLintFloor_ALongFindingListIsCutAndKeepsTheReproduceCommand(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	writeFile(t, filepath.Join(f.in.Worktree, "go", "p", "a.go"), "package p\n\nfunc F() {}\n")
	var many strings.Builder
	for i := 1; many.Len() <= 2*lintFloorFindingsMax; i++ {
		fmt.Fprintf(&many, "p/a.go:%d:2: Error return value is not checked (errcheck)\n", i)
	}
	lint := &fakeLint{findings: many.String()}

	lines, err := lintFloorJudge(lint.run)(context.Background(), f.in)

	if err != nil || len(lines) != 1 {
		t.Fatalf("lines=%d err=%v, want one RED line", len(lines), err)
	}
	if len(lines[0]) >= many.Len() || !strings.Contains(lines[0], "golangci-lint run ") || !strings.Contains(lines[0], "p/a.go:1:2") {
		t.Errorf("RED line of %d bytes from %d bytes of findings, want it cut with the reproduce command and the first finding", len(lines[0]), many.Len())
	}
}
