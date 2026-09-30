//go:build acs

package cycle1334

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evalSlug = "pipeline-defect-consumption-fingerprint-ack"
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
			t.Errorf("test %s did not report PASS in %s (missing, unimplemented, or regressed)", name, pkg)
		}
	}
}

func TestC1334_001_parse_consumption_fingerprint_from_consumed_by(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestParseConsumptionFingerprint_ExtractsFromUnquotedConsumedBy$",
		[]string{"TestParseConsumptionFingerprint_ExtractsFromUnquotedConsumedBy"})
}

func TestC1334_002_parse_consumption_fingerprint_from_notes(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestParseConsumptionFingerprint_ExtractsFromQuotedNotes$",
		[]string{"TestParseConsumptionFingerprint_ExtractsFromQuotedNotes"})
}

func TestC1334_003_parse_consumption_fingerprint_no_match(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestParseConsumptionFingerprint_NoMatchReturnsFalse$",
		[]string{"TestParseConsumptionFingerprint_NoMatchReturnsFalse"})
}

func TestC1334_004_parse_consumption_fingerprint_ignores_unrelated_pipes(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestParseConsumptionFingerprint_IgnoresUnrelatedPipeDelimitedText$",
		[]string{"TestParseConsumptionFingerprint_IgnoresUnrelatedPipeDelimitedText"})
}

func TestC1334_005_consume_pipeline_defect_fingerprint_writes_ledger(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestConsumePipelineDefectFingerprint_WritesLedgerFromConsumedBy$",
		[]string{"TestConsumePipelineDefectFingerprint_WritesLedgerFromConsumedBy"})
}

func TestC1334_006_consume_pipeline_defect_fingerprint_notes_fallback(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestConsumePipelineDefectFingerprint_FallsBackToNotesWhenConsumedByEmpty$",
		[]string{"TestConsumePipelineDefectFingerprint_FallsBackToNotesWhenConsumedByEmpty"})
}

func TestC1334_007_consume_pipeline_defect_fingerprint_errors_without_match(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestConsumePipelineDefectFingerprint_ErrorsWhenNoFingerprintFound$",
		[]string{"TestConsumePipelineDefectFingerprint_ErrorsWhenNoFingerprintFound"})
}

func TestC1334_008_consume_pipeline_defect_fingerprint_integrates_rule_b(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestConsumePipelineDefectFingerprint_IntegratesWithRuleBExclusion$",
		[]string{"TestConsumePipelineDefectFingerprint_IntegratesWithRuleBExclusion"})
}

func TestC1334_009_run_inbox_ack_fingerprint_writes_ledger(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestRunInbox_AckFingerprint_WritesLedgerFromRealItem$",
		[]string{"TestRunInbox_AckFingerprint_WritesLedgerFromRealItem"})
}

func TestC1334_010_run_inbox_ack_fingerprint_notes_fallback(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestRunInbox_AckFingerprint_FallsBackToNotesField$",
		[]string{"TestRunInbox_AckFingerprint_FallsBackToNotesField"})
}

func TestC1334_011_run_inbox_ack_fingerprint_missing_item(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestRunInbox_AckFingerprint_MissingItemReturnsNonZero$",
		[]string{"TestRunInbox_AckFingerprint_MissingItemReturnsNonZero"})
}

func TestC1334_012_run_inbox_ack_fingerprint_no_fingerprint(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestRunInbox_AckFingerprint_NoFingerprintInItemReturnsNonZero$",
		[]string{"TestRunInbox_AckFingerprint_NoFingerprintInItemReturnsNonZero"})
}

func TestC1334_013_eval_file_passes_quality_check(t *testing.T) {
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
