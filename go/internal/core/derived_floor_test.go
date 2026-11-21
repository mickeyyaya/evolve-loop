package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type derivedProbe struct {
	calls    []string
	failing  map[string]error
	worktree string
}

func (p *derivedProbe) forWorktree(worktree string) derived.Generator {
	p.worktree = worktree
	return func(_ context.Context, args ...string) error {
		key := strings.Join(args, " ")
		p.calls = append(p.calls, key)
		return p.failing[key]
	}
}

func exitFailure(t *testing.T, output string) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit 3").Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("fixture exit error = %v", err)
	}
	return fmt.Errorf("%w: %s", err, output)
}

func derivedFloorReview(t *testing.T, f baseRedFixture, probe *derivedProbe, files map[string]string) ReviewResult {
	t.Helper()
	for rel, body := range files {
		writeFile(t, filepath.Join(f.in.Worktree, filepath.FromSlash(rel)), body)
	}
	return BuildHandoffFloor{{Name: "derived", Run: DerivedFloorChecks(probe.forWorktree)}}.Review(context.Background(), f.in)
}

func TestDerivedFloor_ANewPackageWithoutDarwinSetNamesTheGenerateVerb(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	probe := &derivedProbe{failing: map[string]error{"ci darwin-set check": exitFailure(t, "stale")}}

	res := derivedFloorReview(t, f, probe, map[string]string{"go/cmd/newpkg/main.go": "package main\n\nfunc main() {}\n"})

	if res.Approve || !res.Retry {
		t.Fatalf("Approve=%v Retry=%v, want a refusal the builder can correct", res.Approve, res.Retry)
	}
	for _, want := range []string{"evolve ci darwin-set generate", "stage its outputs (go/.darwin-set)", ": stale)"} {
		if !strings.Contains(res.Reason, want) {
			t.Fatalf("reason %q lacks %q", res.Reason, want)
		}
	}
	if probe.worktree != f.in.Worktree {
		t.Fatalf("generator bound to %q, want the lane worktree %q", probe.worktree, f.in.Worktree)
	}
}

func TestDerivedFloor_ACleanDiffYieldsNoLine(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	probe := &derivedProbe{}

	res := derivedFloorReview(t, f, probe, map[string]string{"go/cmd/newpkg/main.go": "package main\n\nfunc main() {}\n"})

	if !res.Approve || len(probe.calls) == 0 {
		t.Fatalf("Approve=%v calls=%v, want a pass that did run the darwin-set check", res.Approve, probe.calls)
	}
}

func TestDerivedFloor_RunsOnlyTheChecksOfTouchedEntries(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	probe := &derivedProbe{}

	derivedFloorReview(t, f, probe, map[string]string{"docs/research-index/2026-11-21-x.md": "## X\n"})

	if len(probe.calls) != 1 || probe.calls[0] != "docs index check" {
		t.Fatalf("calls = %v, want only the research-index check", probe.calls)
	}
}

func TestDerivedFloor_ABrokenResearchEntryNamesTheInputTheCheckReports(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	probe := &derivedProbe{failing: map[string]error{"docs index check": exitFailure(t, `entry 2026-11-21-x.md must start with a "## " heading line`)}}

	res := derivedFloorReview(t, f, probe, map[string]string{"docs/research-index/2026-11-21-x.md": "\n## X\n"})

	if res.Approve || !strings.Contains(res.Reason, "2026-11-21-x.md") {
		t.Fatalf("Approve=%v reason=%q, want a refusal that names the entry file", res.Approve, res.Reason)
	}
}

func TestDerivedFloor_ACheckThatCannotRunIsFailOpenAndWarns(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	probe := &derivedProbe{failing: map[string]error{"ci darwin-set check": errors.New("go: executable not found")}}

	res := derivedFloorReview(t, f, probe, map[string]string{"go/cmd/newpkg/main.go": "package main\n"})

	if !res.Approve {
		t.Fatalf("reason=%q, want a pass: a check that cannot run is not a finding", res.Reason)
	}
	if !f.sawAtSeverity(CodeBuildDerivedFloorSkipped, signalcenter.SeverityWarn) || !strings.Contains(f.stderr.String(), "WARN") {
		t.Fatalf("signals %v, stderr %q: want the skipped signal and a WARN", f.codes(), f.stderr.String())
	}
}

func TestStaleDerivedFindings_ADeadlineIsAnErrorNotAFinding(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run := func(ctx context.Context, _ ...string) error {
		<-ctx.Done()
		return exitFailure(t, "signal: killed")
	}

	lines, err := StaleDerivedFindings(ctx, t.TempDir(), []string{"docs/research-index/a.md"}, run, &injectedStderr{})

	if err == nil || len(lines) != 0 {
		t.Fatalf("lines=%v err=%v, want an error and no finding", lines, err)
	}
}

func TestStaleDerivedFindings_AFailedGoListFallsBackToPathInputsWithANeutralWarn(t *testing.T) {
	t.Parallel()
	warn := &injectedStderr{}
	run := func(context.Context, ...string) error { return nil }

	_, err := StaleDerivedFindings(context.Background(), t.TempDir(), []string{"go/x.go"}, run, warn)

	if err != nil || !strings.Contains(warn.String(), "derived") || strings.Contains(warn.String(), "build-derived-regen") {
		t.Fatalf("err=%v warn=%q, want an origin-neutral derived WARN", err, warn.String())
	}
}

func TestDerivedFloor_AFindingKeepsOnlyTheLastFourHundredBytesOfTheCheckOutput(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	probe := &derivedProbe{failing: map[string]error{"ci darwin-set check": exitFailure(t, "HEAD-PREAMBLE"+strings.Repeat("x", 900)+"go/cmd/newpkg/main.go")}}

	res := derivedFloorReview(t, f, probe, map[string]string{"go/cmd/newpkg/main.go": "package main\n"})

	if res.Approve || strings.Contains(res.Reason, "HEAD-PREAMBLE") || !strings.Contains(res.Reason, "go/cmd/newpkg/main.go") || !strings.Contains(res.Reason, "evolve ci darwin-set generate") {
		t.Fatalf("Approve=%v reason=%q, want a refusal that keeps the tail of the output, where the file is named, and keeps the generate verb", res.Approve, res.Reason)
	}
}

func TestDerivedFloor_AWorktreeWithNoChangedPathRunsNoCheck(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	probe := &derivedProbe{}

	res := derivedFloorReview(t, f, probe, nil)

	if !res.Approve || len(probe.calls) != 0 {
		t.Fatalf("Approve=%v calls=%v, want a pass with no check run", res.Approve, probe.calls)
	}
}

func TestDerivedFloor_AWorktreeGitCannotReadIsFailOpenAndWarns(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	f.in.Worktree = t.TempDir()
	probe := &derivedProbe{}

	res := BuildHandoffFloor{{Name: "derived", Run: DerivedFloorChecks(probe.forWorktree)}}.Review(context.Background(), f.in)

	if !res.Approve || len(probe.calls) != 0 || !strings.Contains(f.stderr.String(), "WARN derived") {
		t.Fatalf("Approve=%v calls=%v stderr=%q, want a fail-open WARN", res.Approve, probe.calls, f.stderr.String())
	}
}

var _ DerivedRunFn = (&derivedProbe{}).forWorktree

func TestStaleDerivedFindings_EachCheckRunsUnderTheDerivedCheckDeadline(t *testing.T) {
	t.Parallel()
	var left time.Duration
	run := func(ctx context.Context, _ ...string) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("the check runs with no deadline")
		}
		left = time.Until(deadline)
		return nil
	}

	_, err := StaleDerivedFindings(context.Background(), t.TempDir(), []string{"docs/research-index/a.md"}, run, &injectedStderr{})

	if err != nil || left <= 0 || left > DerivedCheckDeadline {
		t.Fatalf("err=%v left=%v, want a deadline within %v", err, left, DerivedCheckDeadline)
	}
}

func TestLaneDerivedGenerator_RunsTheEvolveVerbInTheLaneWorktree(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "no-such-worktree")

	err := LaneDerivedGenerator(missing)(context.Background(), "docs", "index", "check")

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want the go run failure of the lane worktree path", err)
	}
}
