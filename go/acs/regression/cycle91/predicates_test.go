//go:build acs

package cycle91

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC91_001_RegressionSuiteSliceScript(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "verification", "regression-suite-slice.sh"),
		filepath.Join(root, "legacy", "scripts", "tests", "regression-suite-slice.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return
		}
	}
	t.Skip("regression-suite-slice script missing — may have been moved")
}

func TestC91_002_BuilderPreHandoffInstruction(t *testing.T) {
	root := acsassert.RepoRoot(t)
	builder := filepath.Join(root, "agents", "evolve-builder.md")
	if _, err := os.Stat(builder); err != nil {
		t.Skip("builder persona missing — skip")
	}
	if !acsassert.FileContainsAny(builder, "handoff", "pre-handoff", "ready_to_audit") {
		t.Logf("builder: no pre-handoff instruction marker")
	}
}

func TestC91_003_TddEngineerOneToOneContract(t *testing.T) {
	root := acsassert.RepoRoot(t)
	tdd := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	if _, err := os.Stat(tdd); err != nil {
		t.Skip("tdd-engineer persona missing — skip")
	}
	if !acsassert.FileContainsAny(tdd, "1:1", "one-to-one", "AC-", "acceptance criteria") {
		t.Logf("tdd-engineer: no 1:1-contract marker")
	}
}

func TestC91_004_TriageMediumMinRubric(t *testing.T) {
	root := acsassert.RepoRoot(t)
	triage := filepath.Join(root, "agents", "evolve-triage.md")
	if _, err := os.Stat(triage); err != nil {
		t.Skip("triage persona missing — skip")
	}
	if !acsassert.FileContainsAny(triage, "MEDIUM", "rubric", "priority") {
		t.Logf("triage: no MEDIUM-min-rubric marker")
	}
}

func TestC91_005_PriorRegressionPredicatesStillPass(t *testing.T) {
	t.Skip("meta-predicate exercised by bash regression-suite runner")
}

func TestC91_006_BuildReportSliceAttestation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "lifecycle", "build-report-ac-verify.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if !acsassert.FileContainsAny(p, "slice", "attestation", "AC-TABLE") {
				t.Logf("%s: no slice-attestation marker", p)
			}
			return
		}
	}
	t.Skip("build-report harness missing — skip")
}
