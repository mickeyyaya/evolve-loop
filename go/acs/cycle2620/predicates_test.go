//go:build acs

package cycle2620

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	guardTest   = "TestStageEvidenceRoutedSourceGuard"
	guardPkg    = "./internal/stageevidence"
	rootFlag    = "-routed-guard-root"
	pendingItem = "F4"
)

func runGuard(t *testing.T, fixtureRoot string) (string, int) {
	t.Helper()
	args := []string{"test", "-count=1", "-v", "-run", "^" + guardTest + "$", guardPkg}
	if fixtureRoot != "" {
		args = append(args, "-args", rootFlag+"="+fixtureRoot)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = acsassert.GoModuleDir(t)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("go test did not run: %v", err)
		}
		code = ee.ExitCode()
	}
	return string(out), code
}

func fixtureTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func requireGuardFails(t *testing.T, files map[string]string, siteFile string) {
	t.Helper()
	out, code := runGuard(t, fixtureTree(t, files))
	if !strings.Contains(out, "--- FAIL: "+guardTest) {
		t.Fatalf("RED: the guard did not fail on a tree whose emit site fills the routed source (exit %d):\n%s", code, acsassert.OutputTail(out, 20))
	}
	if code == 0 {
		t.Errorf("RED: the guard printed a failure but go test exited 0")
	}
	if !strings.Contains(out, siteFile) {
		t.Errorf("RED: the guard failure does not name the emit-site file %s:\n%s", siteFile, acsassert.OutputTail(out, 20))
	}
	if !strings.Contains(out, pendingItem) {
		t.Errorf("RED: the guard failure does not name the F-item %s that will add the source:\n%s", pendingItem, acsassert.OutputTail(out, 20))
	}
}

const outcomeLiteralFillsRouted = `package emit

import stageevidence "github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"

func Seal(id string) stageevidence.Outcome {
	return stageevidence.Outcome{ID: 7, Shipped: true, Routed: []string{id}}
}
`

const outcomeAssignmentFillsRouted = `package emit

import stageevidence "github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"

func Seal(id string) stageevidence.Outcome {
	var out stageevidence.Outcome
	out.Routed = append(out.Routed, id)
	return out
}
`

const heldProposalLiteralCarriesItem = `package emit

import stageevidence "github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"

func Hold(id string) stageevidence.Record {
	return stageevidence.Record{Key: "held_proposal.stage", Fields: map[string]string{"would_hold": "true", "item": id}}
}
`

const heldProposalAssignmentCarriesItem = `package emit

import stageevidence "github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"

func Hold(id string) stageevidence.Record {
	rec := stageevidence.Record{Key: "held_proposal.stage", Fields: map[string]string{"would_hold": "true"}}
	rec.Fields["item"] = id
	return rec
}
`

const cleanEmitSite = `package emit

import stageevidence "github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"

func Seal() stageevidence.Outcome {
	return stageevidence.Outcome{ID: 7, Shipped: true, Failed: []string{"lint"}}
}

func Hold() stageevidence.Record {
	return stageevidence.Record{Key: "held_proposal.stage", Fields: map[string]string{"would_hold": "true"}}
}
`

const testFileMayFillRouted = `package emit

import (
	"testing"

	stageevidence "github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

func TestScoring(t *testing.T) {
	_ = stageevidence.Outcome{ID: 1, Routed: []string{"item-9"}}
	_ = stageevidence.Record{Fields: map[string]string{"would_hold": "true", "item": "item-9"}}
}
`

func TestC2620_001_GuardPassesOnCurrentTreeAndNamesTheFItem(t *testing.T) {
	out, code := runGuard(t, "")
	if code != 0 || !strings.Contains(out, "--- PASS: "+guardTest) {
		t.Fatalf("RED: %s does not pass on the current tree (exit %d):\n%s", guardTest, code, acsassert.OutputTail(out, 20))
	}
	if !strings.Contains(out, pendingItem) {
		t.Errorf("RED: the passing guard run does not name the F-item %s that will add the source:\n%s", pendingItem, acsassert.OutputTail(out, 20))
	}
}

func TestC2620_002_GuardFailsWhenAnOutcomeLiteralFillsRouted(t *testing.T) {
	requireGuardFails(t, map[string]string{"emit/seal.go": outcomeLiteralFillsRouted}, "seal.go")
}

func TestC2620_003_GuardFailsWhenAnAssignmentFillsOutcomeRouted(t *testing.T) {
	requireGuardFails(t, map[string]string{"emit/sealassign.go": outcomeAssignmentFillsRouted}, "sealassign.go")
}

func TestC2620_004_GuardFailsWhenAHeldProposalRecordCarriesAnItem(t *testing.T) {
	requireGuardFails(t, map[string]string{"emit/hold.go": heldProposalLiteralCarriesItem}, "hold.go")
}

func TestC2620_005_GuardFailsWhenAHeldProposalItemIsAssigned(t *testing.T) {
	requireGuardFails(t, map[string]string{"emit/holdassign.go": heldProposalAssignmentCarriesItem}, "holdassign.go")
}

func TestC2620_006_GuardPassesOnEmitSitesThatLeaveTheSourcesEmpty(t *testing.T) {
	root := fixtureTree(t, map[string]string{
		"emit/clean.go":      cleanEmitSite,
		"emit/clean_test.go": testFileMayFillRouted,
	})
	out, code := runGuard(t, root)
	if code != 0 || !strings.Contains(out, "--- PASS: "+guardTest) {
		t.Fatalf("RED: the guard refused a tree that fills no routed source, or a test file that fills one (exit %d):\n%s", code, acsassert.OutputTail(out, 20))
	}
}
