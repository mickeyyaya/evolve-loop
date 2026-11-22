//go:build acs

package cycle2603

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modulePath = "github.com/mickeyyaya/evolve-loop/go/"

func runNamedTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", pkg)
	out := stdout + "\n" + stderr
	if err != nil || code != 0 {
		t.Fatalf("RED: go test %s exited %d (%v)\n%s", pkg, code, err, acsassert.OutputTail(out, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s\n%s", name, pkg, acsassert.OutputTail(out, 40))
		}
	}
}

func funcsCoveredFully(t *testing.T, pkg, runRegex string, funcs ...string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	profile := filepath.Join(t.TempDir(), "cover.out")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-coverprofile="+profile, "-run", runRegex, pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: go test %s exited %d (%v)\n%s", pkg, code, err, acsassert.OutputTail(stdout+"\n"+stderr, 40))
	}
	report, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("RED: go tool cover exited %d (%v)\n%s", code, err, stderr)
	}
	for _, fn := range funcs {
		pct, err := acsassert.FuncCoverage(report, fn)
		if err != nil {
			t.Errorf("RED: %v", err)
			continue
		}
		if pct != 100.0 {
			t.Errorf("RED: %s in %s has %.1f%% statement coverage, want 100%%", fn, pkg, pct)
		}
	}
}

func TestC2603_001_AGrantedScopeWhoseInboxItemIsGoneIsNotPutFirst(t *testing.T) {
	runNamedTests(t, modulePath+"cmd/evolve",
		"TestPausedPlan_AGrantedScopeWhoseItemLeftTheInboxIsNotPutFirstEvenWithARecordedSpec",
		"TestPausedPlan_AGrantedScopeWhoseInboxItemIsGoneIsNotPutFirst",
		"TestPausedPlan_APartlyGoneScopeIsNotPutFirstAndWarns")
}

func TestC2603_002_EachChangedFunctionHasFullStatementCoverage(t *testing.T) {
	funcsCoveredFully(t, modulePath+"cmd/evolve", "TestPausedPlan|TestProductionPoolPlanFn|TestParked|TestInboxAdoptable|TestInboxItemFiles", "lead")
	funcsCoveredFully(t, modulePath+"internal/core", "TestPausedLaneScopes|TestCycleRecord|TestRunCycle_A|TestLaneScopePin", "PausedLaneScopes", "cycleRecord")
	funcsCoveredFully(t, modulePath+"internal/fleet", "TestPoolSpec", "poolSpec")
	funcsCoveredFully(t, modulePath+"internal/committedset", "TestLaneSpec|TestDecodeLaneSpec|TestRecord", "Encode", "DecodeLaneSpec", "IsZero")
}

// acs-predicate: config-check
func TestC2603_003_CheckpointResumeDocNamesTheRecordedSpec(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "checkpoint-resume.md")
	acsassert.FileNotContains(t, doc, "Its dependencies and its output contract are not recorded yet.")
	acsassert.FileContains(t, doc, "LaneSpec")
}
