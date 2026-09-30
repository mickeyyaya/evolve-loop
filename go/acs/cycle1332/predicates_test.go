//go:build acs

package cycle1332

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evalSlug = "blocker-breaker-fingerprint-ack"
	corePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	cmdPkg   = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func requireNamedPass(t *testing.T, pkg, pattern string, names []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, pkg)
	if code != 0 || err != nil {
		t.Logf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, pkg, code, err, stdout, stderr)
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS in %s (missing, renamed, or not implemented yet)", name, pkg)
		}
	}
}

func TestC1332_001_load_resolved_fingerprints_reads_ledger(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestLoadResolvedFingerprints_ReadsLedgerRecords$",
		[]string{"TestLoadResolvedFingerprints_ReadsLedgerRecords"})
}

func TestC1332_002_load_resolved_fingerprints_missing_file_is_empty(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestLoadResolvedFingerprints_MissingFileReturnsEmptyNoError$",
		[]string{"TestLoadResolvedFingerprints_MissingFileReturnsEmptyNoError"})
}

func TestC1332_003_evaluate_blocker_breaker_excludes_acked_fingerprint(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestEvaluateBlockerBreaker_ExcludesAckedFingerprint$",
		[]string{"TestEvaluateBlockerBreaker_ExcludesAckedFingerprint"})
}

func TestC1332_004_evaluate_blocker_breaker_unacked_still_halts(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestEvaluateBlockerBreaker_UnackedIdenticalFingerprintStillHalts$",
		[]string{"TestEvaluateBlockerBreaker_UnackedIdenticalFingerprintStillHalts"})
}

func TestC1332_005_append_resolved_fingerprint_writes_record(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestAppendResolvedFingerprint_WritesRecord$",
		[]string{"TestAppendResolvedFingerprint_WritesRecord"})
}

func TestC1332_006_run_loop_fingerprint_flag_appends_ledger(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestRunLoop_FingerprintAck_AppendsLedgerRecord$",
		[]string{"TestRunLoop_FingerprintAck_AppendsLedgerRecord"})
}

func TestC1332_007_blocker_breaker_halt_acked_fingerprint_does_not_rehalt(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestBlockerBreakerHalt_AckedFingerprintDoesNotReHalt$",
		[]string{"TestBlockerBreakerHalt_AckedFingerprintDoesNotReHalt"})
}

func TestC1332_008_eval_file_passes_quality_check(t *testing.T) {
	evalPath := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", evalSlug+".md")
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
		t.Fatalf("eval %s overall level %d, want PASS(0)", evalPath, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Fatalf("eval %s has %d classifiable command(s), want >=2 (vacuous-empty-eval guard)", evalPath, len(res.Commands))
	}
}
