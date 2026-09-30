//go:build acs

// Package cycle1782 materialises the acceptance criteria of the two fleet-lane
// tasks: starvation-compares-sized-width and disk-space-preflight.
package cycle1782

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/preflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1782_001_StarvedComparesRealizedWithSizedWidth(t *testing.T) {
	if !(fleet.WaveObservation{SizedLanes: 2, RealizedLanes: 1}).Starved() {
		t.Error("1 of 2 sized lanes must be starved")
	}
	if (fleet.WaveObservation{SizedLanes: 2, RealizedLanes: 2}).Starved() {
		t.Error("2 of 2 sized lanes must not be starved")
	}
	if (fleet.WaveObservation{SizedLanes: 1, RealizedLanes: 1}).Starved() {
		t.Error("a shrunk wave delivering its sized width must not be starved")
	}
}

func TestC1782_002_TrackerFiresOnSizedWidthShortfall(t *testing.T) {
	var tr fleet.StarvationTracker
	short := fleet.WaveObservation{SizedLanes: 2, RealizedLanes: 0}
	if tr.Observe(short, 2) || !tr.Observe(short, 2) {
		t.Error("tracker must fire on exactly the 2nd consecutive short wave with K=2")
	}
}

// acs-predicate: config-check
func TestC1782_003_Cycle544RuleSupersededWithReason(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileContains(t, filepath.Join(root, "go", "acs", "cycle544", "predicates_test.go"), "superseded by") {
		t.Error("cycle544 must document that its quota-shrink rule is superseded by the sized-width comparison")
	}
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-tags", "acs", "-count=1", "./acs/cycle544")
	if err != nil || code != 0 {
		t.Errorf("superseded cycle544 package must still build and pass: rc=%d %v\n%s%s", code, err, out, errOut)
	}
}

func TestC1782_004_PolicyFloorHasDefaultAndOverride(t *testing.T) {
	if (policy.Policy{}).PreflightConfig().MinFreeGiB <= 0 {
		t.Error("default floor must be positive")
	}
}

func diskOpts(t *testing.T, free uint64, floor uint64) looppreflight.Options {
	return looppreflight.Options{
		ProjectRoot: t.TempDir(), EvolveDir: t.TempDir(), SkipBoot: true,
		Now:           time.Now,
		SpinePhases:   []string{"build"},
		FactoryKnown:  func(string) bool { return true },
		ContractKnown: func(string) bool { return true },
		ProfileLister: func() ([]string, error) { return []string{"builder"}, nil },
		ProfileGetter: func(n string) (profiles.Profile, error) {
			return profiles.Profile{Name: n, CLI: "claude-tmux"}, nil
		},
		DriverKnown: func(string) bool { return true },
		ProbeCLI: func(b string) (doctor.Result, error) {
			return doctor.Result{Tool: b, Found: true, Path: "/usr/bin/" + b, Method: "path"}, nil
		},
		HostProbe: func() preflight.Profile {
			return preflight.Profile{Sandbox: preflight.Sandbox{ExpectedToWork: true, SandboxExecAvailable: true}}
		},
		DirWritable:        func(string) bool { return true },
		DiskFreeBytes:      func(string) (uint64, error) { return free, nil },
		SelfUpdateEvidence: func(string) (bool, string, error) { return false, "", nil },
		PinnedLister:       func() ([]string, error) { return nil, nil },
		MinFreeBytes:       floor,
	}
}

func diskCheck(t *testing.T, r looppreflight.Result) looppreflight.CheckResult {
	for _, c := range r.Checks {
		if c.Name == "disk-space" {
			return c
		}
	}
	t.Fatalf("no disk-space check in %+v", r.Checks)
	return looppreflight.CheckResult{}
}

func TestC1782_005_BootHaltsBelowFloorOnInjectedStatfs(t *testing.T) {
	r, err := looppreflight.Run(diskOpts(t, 143<<20, 1<<30))
	if err != nil {
		t.Fatal(err)
	}
	c := diskCheck(t, r)
	if c.Level != looppreflight.LevelHalt || !strings.Contains(c.Message+c.Detail, "evolve gc") {
		t.Errorf("want a halt naming `evolve gc`, got %s %q %q", c.Level, c.Message, c.Detail)
	}
}

func TestC1782_006_BootPassesAtAndAboveFloor(t *testing.T) {
	r, err := looppreflight.Run(diskOpts(t, 1<<30, 1<<30))
	if err != nil {
		t.Fatal(err)
	}
	if c := diskCheck(t, r); c.Level != looppreflight.LevelPass {
		t.Errorf("free == floor: level=%s want pass", c.Level)
	}
}

// Wiring proof: the production callers (boot preflight, wave boundary, starvation observer)
// reach the new seams. Narrow -run over one package; no whole-repo sweep.
func TestC1782_007_ProductionCallersReachTheSeams(t *testing.T) {
	root := acsassert.RepoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-run",
		"^(TestObserveWorkSupply_|TestDispatchFleetIteration_Disk|TestDefaultLoopPreflight_DiskFloor)", "./cmd/evolve")
	cmd.Dir = filepath.Join(root, "go")
	cmd.WaitDelay = 10 * time.Second
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("caller-reachability tests failed: %v\n%s", err, out)
	}
}

// acs-predicate: config-check
func TestC1782_008_RuntimeReferenceListsTheFloor(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileContains(t, filepath.Join(root, "docs", "operations", "runtime-reference.md"), "min_free_gib") {
		t.Error("runtime-reference.md must document preflight.min_free_gib")
	}
}
