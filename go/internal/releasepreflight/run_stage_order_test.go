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

type seamCounters struct {
	gitClean, branch, headSHA, gate, nameGuard, ci, simulation int
}

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
				return "", nil
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
		if err := os.Remove(filepath.Join(repo, ".claude-plugin", "plugin.json")); err != nil {
			t.Fatal(err)
		}
	case "step3b":
		o.Target = "1.0.0"
	case "step4":
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
		o.SkipTests = false
	}
	return o, c
}

var errTestFailure = errors.New("gate suite blew up")

var tsField = regexp.MustCompile(`"ts":"[^"]*"`)

func TestRun_FirstFailingStepWins(t *testing.T) {
	for _, tc := range []struct {
		failAt      string
		wantErr     string
		unreachable func(*seamCounters) (string, int)
	}{
		{"step1", "uncommitted", func(c *seamCounters) (string, int) { return "branch", c.branch }},
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
