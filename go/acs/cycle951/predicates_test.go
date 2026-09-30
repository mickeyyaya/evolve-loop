//go:build acs

package cycle951

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	coherencePkg = "github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	corePkg      = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	modulePkg    = "github.com/mickeyyaya/evolve-loop/go/..."

	taskSlug = "coherence-reconcile-selfheal"
)

func signatureInputs(rec string, deliverableValid bool) coherence.VerdictInputs {
	return coherence.VerdictInputs{
		Recorded:         rec,
		Audit:            "PASS",
		ACS:              "PASS",
		AuditRan:         true,
		SubstantiveError: false,
		DeliverableValid: deliverableValid,
	}
}

func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runExpr, "-v", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s", name, stdout)
		}
	}
}

func TestC951_001_ValidDeliverableReconciles(t *testing.T) {
	for _, rec := range []string{"FAIL", "WARN"} {
		coh := coherence.CheckVerdictCoherence(signatureInputs(rec, true))
		if !coh.Reconciled {
			t.Errorf("recorded=%s + green artifacts + valid deliverable: Reconciled=false, want true (%+v)", rec, coh)
		}
		if coh.Incoherent {
			t.Errorf("recorded=%s + valid deliverable must NOT be Incoherent (no halt), got %+v", rec, coh)
		}
	}
}

func TestC951_002_InvalidDeliverableStillHalts(t *testing.T) {
	for _, rec := range []string{"FAIL", "WARN"} {
		coh := coherence.CheckVerdictCoherence(signatureInputs(rec, false))
		if !coh.Incoherent {
			t.Errorf("recorded=%s + green artifacts + INVALID deliverable: Incoherent=false, want true (forged verdict must halt) (%+v)", rec, coh)
		}
		if coh.Reconciled {
			t.Errorf("recorded=%s + INVALID deliverable must NOT reconcile (would launder forgery to PASS), got %+v", rec, coh)
		}
		if coh.Category != "verdict-incoherence" {
			t.Errorf("recorded=%s category = %q, want verdict-incoherence", rec, coh.Category)
		}
	}
}

func TestC951_003_ValidDeliverableNeverManufacturesReconcile(t *testing.T) {
	cases := []struct {
		name string
		in   coherence.VerdictInputs
	}{
		{"recorded PASS", coherence.VerdictInputs{Recorded: "PASS", Audit: "PASS", ACS: "PASS", AuditRan: true, DeliverableValid: true}},
		{"substantive error explains negative", coherence.VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "PASS", AuditRan: true, SubstantiveError: true, DeliverableValid: true}},
		{"audit never ran", coherence.VerdictInputs{Recorded: "FAIL", Audit: "", ACS: "PASS", AuditRan: false, DeliverableValid: true}},
		{"acs artifact absent", coherence.VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "", AuditRan: true, DeliverableValid: true}},
	}
	for _, c := range cases {
		coh := coherence.CheckVerdictCoherence(c.in)
		if coh.Reconciled {
			t.Errorf("%s: Reconciled=true — a coherent case must not reconcile just because the deliverable is valid (%+v)", c.name, coh)
		}
		if coh.Incoherent {
			t.Errorf("%s: Incoherent=true — coherent case must stay coherent (%+v)", c.name, coh)
		}
	}
}

func TestC951_004_CoherenceUnitTestsGreen(t *testing.T) {
	runGoTest(t, coherencePkg,
		"^TestCheckVerdictCoherence_(Reconcile|ForgedStillHalts)$",
		[]string{
			"TestCheckVerdictCoherence_Reconcile",
			"TestCheckVerdictCoherence_ForgedStillHalts",
		})
}

func TestC951_005_CallSiteUsesFullVerify(t *testing.T) {
	runGoTest(t, corePkg,
		"^TestDetectVerdictIncoherence_ReconcileUsesFullVerify$",
		[]string{"TestDetectVerdictIncoherence_ReconcileUsesFullVerify"})
}

func TestC951_006_ChangedLogicRaceClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1",
		"-run", "^Test(CheckVerdictCoherence|DetectVerdictIncoherence)",
		coherencePkg, corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race (coherence+detect) exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
}

func TestC951_007_ModuleBuilds(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			modulePkg, code, err, stdout, stderr)
	}
}

func TestC951_008_VetTouchedPackagesClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", coherencePkg, corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			coherencePkg, corePkg, code, err, stdout, stderr)
	}
}

func TestC951_009_EvalFilePassesQualityCheck(t *testing.T) {
	root := acsassert.RepoRoot(t)
	evalPath := filepath.Join(root, ".evolve", "evals", taskSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", taskSlug, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Errorf("eval %s classified only %d command(s) — a vacuous eval is not a PASS", taskSlug, len(res.Commands))
	}
}
