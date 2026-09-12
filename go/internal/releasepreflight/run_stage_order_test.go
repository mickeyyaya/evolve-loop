package releasepreflight

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

// run_stage_order_test.go characterizes three properties of Run that the
// existing suite leaves unpinned, all of which a stage-table extraction could
// silently break:
//
//  1. ORDER — which step fails FIRST when several would fail. The existing
//     failure tests each poison one step against an otherwise-valid repo, so
//     none of them proves step 1 runs before step 3 reads plugin.json.
//  2. StepsPassed ON FAILURE — Result's doc comment promises it is "populated
//     even on failure for diagnostics", but every existing assertion is
//     against a SUCCESSFUL run (== 5), so the counter could stop advancing on
//     the failure path unnoticed.
//  3. DRY-RUN REACH of step 3 — it is the one step with no dry-run branch, and
//     nothing currently proves that is deliberate rather than an oversight.
//
// These are characterization tests: they assert what Run does TODAY, so the
// refactor that follows is provably behavior-preserving.

// seamCounters records which seams a Run actually reached. A step that must
// not have been reached has a zero count — that is the order proof.
type seamCounters struct {
	gitClean, branch, headSHA, gate, nameGuard, ci, simulation int
}

// poisonedOpts builds a repo whose every seam AFTER failAt would fail loudly
// if reached, with the named step rigged to fail. Steps before failAt pass.
// The counters let a test assert not just WHICH error surfaced but that the
// later seams were never consulted.
func poisonedOpts(t *testing.T, failAt string) (Options, *seamCounters) {
	t.Helper()
	repo := makeRepo(t, "1.0.0")
	c := &seamCounters{}

	o := Options{
		Target:    "1.0.1",
		RepoRoot:  repo,
		SkipTests: true,
		Now:       func() time.Time { return time.Now() },
		GitClean: func(string) (bool, error) {
			c.gitClean++
			return failAt != "step1", nil
		},
		CurrentBranch: func(string) (string, error) {
			c.branch++
			if failAt == "step2" {
				return "", nil // detached
			}
			return "main", nil
		},
		HeadSHA: func(string) (string, error) {
			c.headSHA++
			return "deadbeef", nil
		},
		GateTestRunner: func(string, string) error {
			c.gate++
			if failAt == "step5" {
				return errTestFailure
			}
			return nil
		},
		NameGuard: func(string) ([]naminguard.Violation, error) {
			c.nameGuard++
			if failAt == "step5b" {
				return []naminguard.Violation{{File: "README.md", Line: 1, Token: "dead" + "token"}}, nil
			}
			return nil, nil
		},
		CIConclusion: func(string) (CIRunStatus, error) {
			c.ci++
			if failAt == "ci" {
				return CIRunStatus{Conclusion: "failure", RunURL: "http://x"}, nil
			}
			return CIRunStatus{Conclusion: "success"}, nil
		},
		SimulationRunner: func(string) error {
			c.simulation++
			return nil
		},
	}

	switch failAt {
	case "step3":
		// Remove plugin.json so step 3 cannot read the current version.
		if err := os.Remove(filepath.Join(repo, ".claude-plugin", "plugin.json")); err != nil {
			t.Fatal(err)
		}
	case "step3b":
		o.Target = "1.0.0" // equal to current: nothing to bump
	case "step4":
		// Age the ledger entry past MaxAuditAge so step 4 rejects it. Match the
		// "ts" FIELD rather than a byte offset: an offset would silently rewrite
		// the wrong bytes (never erroring, since the slice you extracted always
		// matches itself) the moment makeRepo reorders its JSON or switches to
		// RFC3339Nano, and the resulting failure would look like an unrelated
		// ledger-parse error rather than a broken fixture.
		stale := time.Now().UTC().Add(-(MaxAuditAge + 48*time.Hour)).Format(time.RFC3339)
		ledgerPath := filepath.Join(repo, ".evolve", "ledger.jsonl")
		body, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatal(err)
		}
		aged, n := tsField.ReplaceAllString(string(body), `"ts":"`+stale+`"`), tsField.FindAllString(string(body), -1)
		if len(n) != 1 {
			t.Fatalf("fixture ledger should carry exactly one ts field, found %d — makeRepo's shape changed", len(n))
		}
		if err := os.WriteFile(ledgerPath, []byte(aged), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if failAt == "step5" || failAt == "step5b" || failAt == "ci" {
		o.SkipTests = false // step 5 must actually run its suites + naming scan
	}
	return o, c
}

// errTestFailure is a plain sentinel, matching how every other stub seam in
// this package signals failure (errors.New, six call sites across
// releasepreflight_test.go and extra_coverage_test.go). It never reaches the
// caller's Unwrap chain — stepGateSuites formats it with %v, not %w — so a
// bespoke error type would buy nothing.
var errTestFailure = errors.New("gate suite blew up")

// tsField matches the ledger entry's timestamp FIELD, so aging a fixture does
// not depend on makeRepo's private JSON field order.
var tsField = regexp.MustCompile(`"ts":"[^"]*"`)

// TestRun_FirstFailingStepWins pins the ORDER of Run's steps at the public
// seam: with every step rigged to fail, the EARLIEST one must be the error
// that surfaces, and no seam belonging to a later step may be consulted.
func TestRun_FirstFailingStepWins(t *testing.T) {
	for _, tc := range []struct {
		failAt      string
		wantErr     string
		unreachable func(*seamCounters) (string, int)
	}{
		{"step1", "uncommitted", func(c *seamCounters) (string, int) { return "branch", c.branch }},
		// "detached HEAD", not the looser "detached": the phrasing reaches the
		// operator verbatim through the CLI's `[preflight] FAIL: %v`, so it is
		// observable behavior, not log wording. A decomposition that rewrites
		// it from memory (this one did, once) must fail here.
		{"step2", "detached HEAD", func(c *seamCounters) (string, int) { return "headSHA", c.headSHA }},
		{"step3", "plugin.json missing", func(c *seamCounters) (string, int) { return "headSHA", c.headSHA }},
		{"step3b", "nothing to bump", func(c *seamCounters) (string, int) { return "headSHA", c.headSHA }},
		{"step4", "re-run Auditor", func(c *seamCounters) (string, int) { return "gate", c.gate }},
		{"step5", "gate-test suite failed", func(c *seamCounters) (string, int) { return "ci", c.ci }},
		{"step5b", "naming token", func(c *seamCounters) (string, int) { return "ci", c.ci }},
		{"ci", "not success", func(c *seamCounters) (string, int) { return "simulation", c.simulation }},
	} {
		t.Run(tc.failAt, func(t *testing.T) {
			opts, counters := poisonedOpts(t, tc.failAt)
			_, err := Run(opts)
			if err == nil {
				t.Fatalf("want a failure at %s, got nil", tc.failAt)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want it to contain %q — an EARLIER step surfaced instead", err, tc.wantErr)
			}
			if name, count := tc.unreachable(counters); count != 0 {
				t.Errorf("%s seam was consulted %d time(s) after %s failed — steps ran out of order", name, count, tc.failAt)
			}
		})
	}
}

// TestRun_StepsPassedOnFailure_CountsStepsReached pins Result's documented
// "populated even on failure for diagnostics" contract: StepsPassed must
// report how far the run actually got, and StepsTotal must always be set.
func TestRun_StepsPassedOnFailure_CountsStepsReached(t *testing.T) {
	for _, tc := range []struct {
		failAt string
		want   int
	}{
		{"step1", 0},
		{"step2", 1},
		{"step3", 2},
		{"step3b", 2},
		{"step4", 3},
		{"step5", 4},
		{"step5b", 4},
		{"ci", 5},
	} {
		t.Run(tc.failAt, func(t *testing.T) {
			opts, _ := poisonedOpts(t, tc.failAt)
			res, err := Run(opts)
			if err == nil {
				t.Fatalf("want a failure at %s, got nil", tc.failAt)
			}
			if res.StepsPassed != tc.want {
				t.Errorf("StepsPassed = %d, want %d (steps completed before %s failed)", res.StepsPassed, tc.want, tc.failAt)
			}
			if res.StepsTotal != 5 {
				t.Errorf("StepsTotal = %d, want 5 even on the failure path", res.StepsTotal)
			}
		})
	}
}

// TestRun_DryRun_SemverBumpStillEnforced pins that step 3 is deliberately the
// one step with NO dry-run branch: a dry run still reads plugin.json and still
// rejects an invalid bump. Every seam is nil, so an accidental seam call would
// surface as a different error (a real git invocation against a non-repo),
// which makes this test sensitive to that too.
func TestRun_DryRun_SemverBumpStillEnforced(t *testing.T) {
	for _, tc := range []struct {
		name, target, wantErr string
		removePlugin          bool
	}{
		{name: "equal version is not a bump", target: "1.0.0", wantErr: "nothing to bump"},
		{name: "downgrade is rejected", target: "0.9.0", wantErr: "not greater than"},
		{name: "missing plugin.json still fails", target: "1.0.1", wantErr: "plugin.json missing", removePlugin: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := makeRepo(t, "1.0.0")
			if tc.removePlugin {
				if err := os.Remove(filepath.Join(repo, ".claude-plugin", "plugin.json")); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Run(Options{Target: tc.target, RepoRoot: repo, DryRun: true})
			if err == nil {
				t.Fatalf("dry run must still enforce the semver bump, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}

	// And the positive case: a VALID bump under dry-run reaches the end with
	// every step counted and the current version recorded.
	res, err := Run(Options{Target: "1.0.1", RepoRoot: makeRepo(t, "1.0.0"), DryRun: true})
	if err != nil {
		t.Fatalf("valid dry run err = %v", err)
	}
	if res.StepsPassed != 5 {
		t.Errorf("StepsPassed = %d, want 5", res.StepsPassed)
	}
	if res.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want 1.0.0 — step 3 must read plugin.json even under dry-run", res.CurrentVersion)
	}
}
