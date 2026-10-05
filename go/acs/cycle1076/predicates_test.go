//go:build acs

package cycle1076

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func fixture(t *testing.T, claimed, present []string) core.ReviewInput {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, "workspace")
	wt := filepath.Join(root, "worktree")
	for _, d := range []string{ws, wt} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	quoted := make([]string, 0, len(claimed))
	for _, p := range claimed {
		quoted = append(quoted, `"`+p+`"`)
	}
	report := "# Build Report\n\n```json\n{\"removedPaths\": [" + strings.Join(quoted, ", ") + "]}\n```\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(report), 0o644); err != nil {
		t.Fatalf("write build-report.md: %v", err)
	}
	for _, p := range present {
		abs := filepath.Join(wt, p)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(abs, []byte("still here\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws, Worktree: wt}
}

func TestC1076_001_FalseRemovalClaimIsDetected(t *testing.T) {
	const p = "go/acs/cycle660/predicates_test.go"

	bad := core.RemovalClaimFailures(context.Background(), fixture(t, []string{p}, []string{p}))
	if len(bad) == 0 {
		t.Fatalf("false removal claim produced no failure — acceptance criterion 2 unmet")
	}
	if !strings.Contains(strings.Join(bad, "\n"), p) {
		t.Errorf("failure does not name the falsely-claimed path: %v", bad)
	}

	good := core.RemovalClaimFailures(context.Background(), fixture(t, []string{p}, nil))
	if len(good) != 0 {
		t.Errorf("honest removal claim produced failures: %v", good)
	}
}

func TestC1076_002_CheckIsWiredIntoProductionFloorEngine(t *testing.T) {
	const p = "stale/scaffold.go"
	got := core.DefaultBuildFloorChecks(context.Background(), fixture(t, []string{p}, []string{p}))
	if len(got) == 0 {
		t.Fatalf("DefaultBuildFloorChecks surfaced nothing — removal-claim check is UNWIRED")
	}
	if !strings.Contains(strings.Join(got, "\n"), p) {
		t.Fatalf("production engine failures do not name the falsely-claimed path: %v", got)
	}
}

func TestC1076_003_FalseClaimRejectsTheBuildDeliverable(t *testing.T) {
	r := core.BuildHandoffFloor{{Name: "default", Run: core.DefaultBuildFloorChecks}}
	const p = "stale/scaffold.go"

	if res := r.Review(context.Background(), fixture(t, []string{p}, []string{p})); res.Approve {
		t.Fatalf("build deliverable APPROVED despite a false removal claim")
	} else if !strings.Contains(res.Reason, p) {
		t.Errorf("reject reason omits the path: %q", res.Reason)
	}

	if res := r.Review(context.Background(), fixture(t, []string{p}, nil)); !res.Approve {
		t.Fatalf("honest build deliverable REJECTED: %q", res.Reason)
	}
}

func TestC1076_004_TopnGateUnitsStillGreen(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1", "./internal/topngate/...")
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("./internal/topngate/... exited %d — Part 1 regressed\n%s", code, out)
	}
}

func TestC1076_006_GoVetClean(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-C", goDir, "./...")
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go vet failed to launch (not a vet finding): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("go vet ./... exited %d\n%s", code, out)
	}
}

func TestC1076_005_CoreUnitsGreenIncludingRemovalCheck(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	const name = "TestBuildFloorReviewer_RemovalClaimNotActuallyRemoved"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1", "-v", "-run", "^"+name+"$", "./internal/core/")
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("./internal/core %s exited %d\n%s", name, code, out)
	}
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("no PASS line for %s (renamed, skipped, or never ran?)\n%s", name, out)
	}
}
