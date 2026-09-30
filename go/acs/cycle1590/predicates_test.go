//go:build acs

package cycle1590

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	gitexecPkg = "github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	swarmPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	cmdPkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	modulePkg  = "github.com/mickeyyaya/evolve-loop/go/..."
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

func TestC1590_001_ValidDeliverableReconcilesNotHalt(t *testing.T) {
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

func TestC1590_002_MalformedDeliverableStillHalts(t *testing.T) {
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

func TestC1590_003_MissingACSNeverReconciles(t *testing.T) {
	cases := []struct {
		name string
		in   coherence.VerdictInputs
	}{
		{"acs absent", coherence.VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "", AuditRan: true, DeliverableValid: true}},
		{"acs malformed (WARN)", coherence.VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "WARN", AuditRan: true, DeliverableValid: true}},
		{"recorded PASS", coherence.VerdictInputs{Recorded: "PASS", Audit: "PASS", ACS: "PASS", AuditRan: true, DeliverableValid: true}},
	}
	for _, c := range cases {
		coh := coherence.CheckVerdictCoherence(c.in)
		if coh.Reconciled {
			t.Errorf("%s: Reconciled=true — must not reconcile without both artifacts green (%+v)", c.name, coh)
		}
		if coh.Incoherent && c.name != "recorded PASS" {
			t.Errorf("%s: Incoherent=true — an absent/non-PASS ACS cannot prove forgery either (%+v)", c.name, coh)
		}
	}
}

func TestC1590_005_CallSiteBindsSubstantiveErrorAndFullVerify(t *testing.T) {
	runGoTest(t, corePkg,
		"^TestDetectVerdictIncoherence_(ShipPhaseExplainedFail_NoHalt|ReconcileUsesFullVerify)$",
		[]string{
			"TestDetectVerdictIncoherence_ShipPhaseExplainedFail_NoHalt",
			"TestDetectVerdictIncoherence_ReconcileUsesFullVerify",
		})
}

func TestC1590_006_ForgedVerdictHaltsLive(t *testing.T) {
	runGoTest(t, corePkg,
		"^TestDetectVerdictIncoherence_ForgedVerdict_Halts$",
		[]string{"TestDetectVerdictIncoherence_ForgedVerdict_Halts"})
}

func TestC1590_004_AllFourEntryPointsRetryTransientFailure(t *testing.T) {
	runGoTest(t, corePkg,
		"^TestGitWorktreeCreate_RetriesTransientAddFailure$",
		[]string{"TestGitWorktreeCreate_RetriesTransientAddFailure"})
	runGoTest(t, corePkg,
		"^TestGitWorktreeCreateFrom_RetriesTransientAddFailure$",
		[]string{"TestGitWorktreeCreateFrom_RetriesTransientAddFailure"})
	runGoTest(t, swarmPkg,
		"^TestSwarmCreateWorker_RetriesTransientAddFailure$",
		[]string{"TestSwarmCreateWorker_RetriesTransientAddFailure"})
}

func TestC1590_007_OperatorCLIUsesSharedRetryHelper(t *testing.T) {
	runGoTest(t, cmdPkg,
		"^TestRunWorktreeCreate_Success$",
		[]string{"TestRunWorktreeCreate_Success"})
}

func TestC1590_008_FirstFailurePreservedOnSecondFailure(t *testing.T) {
	runGoTest(t, gitexecPkg,
		"^TestAddWorktreeWithRetry_PreservesFirstFailure$",
		[]string{"TestAddWorktreeWithRetry_PreservesFirstFailure"})
	runGoTest(t, gitexecPkg,
		"^TestAddWorktreeWithRetry_AnnouncesBeforeBackoff$",
		[]string{"TestAddWorktreeWithRetry_AnnouncesBeforeBackoff"})
}

func TestC1590_009_PermanentFailureFailsFastNoBackoff(t *testing.T) {
	runGoTest(t, gitexecPkg,
		"^TestAddWorktreeWithRetry_PermanentFailureSkipsBackoff$",
		[]string{"TestAddWorktreeWithRetry_PermanentFailureSkipsBackoff"})
	runGoTest(t, corePkg,
		"^TestGitWorktreeCreate_PersistentFailureStillFailsLoudly$",
		[]string{"TestGitWorktreeCreate_PersistentFailureStillFailsLoudly"})
	runGoTest(t, swarmPkg,
		"^TestSwarmCreateWorker_PersistentFailureStillFailsLoudly$",
		[]string{"TestSwarmCreateWorker_PersistentFailureStillFailsLoudly"})
}

func runGoTestRace(t *testing.T, pkg, runExpr string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-run", runExpr, pkg)
	if code != 0 || err != nil {
		t.Errorf("go test -race -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
}

func TestC1590_010_TouchedSurfacesRaceClean(t *testing.T) {
	runGoTestRace(t, "github.com/mickeyyaya/evolve-loop/go/internal/coherence", "^TestCheckVerdictCoherence")
	runGoTestRace(t, corePkg, "^Test(DetectVerdictIncoherence|GitWorktreeCreate)")
	runGoTestRace(t, gitexecPkg, "^TestAddWorktreeWithRetry")
}

func TestC1590_011_ModuleBuilds(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", modulePkg, code, err, stdout, stderr)
	}
}

func TestC1590_012_ModuleVets(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", modulePkg, code, err, stdout, stderr)
	}
}

func TestC1590_013_EvalFilesPassQualityCheck(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, slug := range []string{
		"verdict-surface-clean-exit-binding",
		"worktree-provisioning-retry-boundary",
	} {
		path := root + "/.evolve/evals/" + slug + ".md"
		if !acsassert.FileExists(t, path) {
			t.Fatalf("RED: %s missing on disk", path)
		}
		res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: path})
		if err != nil {
			t.Fatalf("%s: quality-check errored: %v", slug, err)
		}
		if res.Overall != evalqualitycheck.LevelPass {
			t.Errorf("%s: quality-check verdict=%v, want PASS (Level-0 tautology or worse)", slug, res.Overall)
		}
	}
}
