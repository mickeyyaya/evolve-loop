package audit

// ciparity_premove_pins_test.go — ADR-0103 unit 14, step 1: the order and
// byte-identity invariants no test pinned before the CI-parity gates moved
// into internal/phases/audit/ciparitygate. Every pin here was GREEN on the
// pre-extraction code (8e8f080f) and proven RED against its named mutant
// before the move; they run through the kept facades, so the leaf must keep
// them green byte-for-byte. The goldens they read were captured on the same
// tree (ciparitygate/testdata/*.golden.*). The pins that named moved symbols
// (the module guard before derivation, the lock root, the attempt-1 write
// before the lock wait, the two stderr lines) moved with them into the leaf
// (scope, lock, stream and stderr tests) and were deleted here.

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// premoveGolden reads one `key<TAB>quoted` golden file captured on 8e8f080f.
func premoveGolden(t *testing.T, name string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("ciparitygate", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		k, q, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("golden line %q", line)
		}
		v, err := strconv.Unquote(q)
		if err != nil {
			t.Fatal(err)
		}
		out[k] = v
	}
	return out
}

// Pin 2 (M2): the enforce list is read BEFORE the derivable check, so a
// non-git root with go.mod and NO .apicover-enforce is a silent no-op for both
// apicover gates — not the underivable hard-FAIL.
func TestApicoverGates_MissingEnforceListWinsOverUnderivable(t *testing.T) {
	root, _ := goWorktree(t) // go.mod, no .apicover-enforce, not a git repo, no handoff
	req := core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1}
	if off, err := apicoverEnforceChangedDefault(req); off != nil || err != nil {
		t.Errorf("apicover-enforce without an enforce list = (%v, %v), want (nil, nil)", off, err)
	}
	if off, err := apicoverNewPackageGraduationDefault(req); off != nil || err != nil {
		t.Errorf("graduation without an enforce list = (%v, %v), want (nil, nil)", off, err)
	}
}

// Pin 3 (M3, M4, a dropped release): when the serialized retake cannot even
// start, integration-tier.log holds attempt 1 ONLY, the offenders are attempt
// 1's plus the log pointer, and the cross-lane lock is released by the time
// the gate returns.
func TestIntegrationTier_RetakeExecFailure_LogHoldsAttemptOneOnlyAndReleasesFirst(t *testing.T) {
	req := tierFixture(t)
	calls := 0
	withFakeRunner(t, func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		calls++
		if calls == 1 {
			_, _ = io.WriteString(so, "--- FAIL: TestFirstAttempt (0.00s)\nFAIL\tpkg\t1.0s\n")
			return 1, nil
		}
		return 0, errors.New("exec: fork/exec go: resource temporarily unavailable")
	})
	off, err := integrationTierCheckDefault(req)
	if err != nil {
		t.Fatalf("retake exec failure must FAIL on attempt-1 offenders, not WARN: %v", err)
	}
	joined := strings.Join(off, "\n")
	if !strings.Contains(joined, "TestFirstAttempt") || !strings.Contains(joined, "full output: ") {
		t.Errorf("offenders must be attempt 1's plus the log pointer; got %v", off)
	}
	logB, rerr := os.ReadFile(filepath.Join(req.Workspace, "integration-tier.log"))
	if rerr != nil {
		t.Fatal(rerr)
	}
	if !strings.Contains(string(logB), "# attempt 1 (lane env, contended)") || strings.Contains(string(logB), "# attempt 2") {
		t.Errorf("the log must hold attempt 1 only when the retake never ran:\n%s", logB)
	}
	rel, held, lerr := flock.TryLock(filepath.Join(req.ProjectRoot, ".evolve", "locks", "integration-tier.lock"))
	if lerr != nil || held {
		t.Fatalf("the retake lock must be released by the time the gate returns (held=%v, err=%v)", held, lerr)
	}
	rel()
}

// Pin 5 (M6): the retake runs under a FRESH budget — its ctx deadline is later
// than attempt 1's, never the leftovers of the same ctx.
func TestIntegrationTier_RetakeUsesAFreshBudget(t *testing.T) {
	req := tierFixture(t)
	var deadlines []time.Time
	withFakeRunner(t, func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		d, _ := ctx.Deadline()
		deadlines = append(deadlines, d)
		time.Sleep(2 * time.Millisecond) // a coarse clock must still separate the two
		if len(deadlines) == 1 {
			_, _ = io.WriteString(so, "--- FAIL: TestFlaky (0.00s)\n")
			return 1, nil
		}
		return 0, nil
	})
	if _, err := integrationTierCheckDefault(req); err == nil || !strings.Contains(err.Error(), "flake") {
		t.Fatalf("red-then-green must absorb the flake: %v", err)
	}
	if len(deadlines) != 2 || !deadlines[1].After(deadlines[0]) {
		t.Fatalf("the retake must carry a fresh, later deadline: %v", deadlines)
	}
}

// Pin 6 (M7, M8): integration-tier.log is byte-identical to the golden
// captured on 8e8f080f for the red-then-green run.
func TestIntegrationTierLog_MatchesTheGoldenBytes(t *testing.T) {
	req := tierFixture(t)
	fn, _, _ := seqRunFunc(t, []struct {
		Code int
		Out  string
	}{{1, "--- FAIL: TestFlaky (0.00s)\nFAIL\tpkg\t1.0s\n"}, {0, "ok\n"}})
	withFakeRunner(t, fn)
	if _, err := integrationTierCheckDefault(req); err == nil {
		t.Fatal("red-then-green must return the flake WARN")
	}
	got, err := os.ReadFile(filepath.Join(req.Workspace, "integration-tier.log"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("ciparitygate", "testdata", "tier-log-red-green.golden.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("integration-tier.log drifted from the golden:\n got %q\nwant %q", got, want)
	}
}

// Pin 7 (M9, M10): the cycle-581 severity asymmetry, byte-exact — the three
// whole-repo gates WARN with the one underivable text; the two apicover gates
// hard-FAIL with their own D1/D2 sentence.
func TestChangedSetUnderivable_SeverityAsymmetryBytes(t *testing.T) {
	g := premoveGolden(t, "messages.golden.txt")
	root := enforceFixtureNonGit(t)
	req := core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1, Workspace: t.TempDir()}
	for _, w := range wholeRepoGates {
		off, err := w.check(req)
		if off != nil || err == nil || err.Error() != g["changeset.underivable"] {
			t.Errorf("%s: (%v, %v), want (nil, the golden underivable WARN)", w.name, off, err)
		}
	}
	if off, err := apicoverEnforceChangedDefault(req); err != nil || len(off) != 1 || off[0] != g["apicover.underivable"] {
		t.Errorf("apicover-enforce underivable = (%v, %v), want exactly the golden D1 offender", off, err)
	}
	if off, err := apicoverNewPackageGraduationDefault(req); err != nil || len(off) != 1 || off[0] != g["graduation.underivable"] {
		t.Errorf("graduation underivable = (%v, %v), want exactly the golden D2 offender", off, err)
	}
}
