//go:build acs

package cycle1855

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
)

func TestC1855_001_VerdictInputsHasNoWriteOnlyFailReasons(t *testing.T) {
	typ := reflect.TypeOf(coherence.VerdictInputs{})
	if _, found := typ.FieldByName("FailReasons"); found {
		t.Errorf("RED: coherence.VerdictInputs still declares the write-only FailReasons field")
	}
	for _, consumed := range []string{"Recorded", "Audit", "ACS", "AuditRan", "SubstantiveError", "DeliverableValid"} {
		if _, found := typ.FieldByName(consumed); !found {
			t.Errorf("consumed field %s was removed from coherence.VerdictInputs", consumed)
		}
	}
}

func writeWorkspace(t *testing.T, auditReport, acsVerdict string) string {
	t.Helper()
	dir := t.TempDir()
	if auditReport != "" {
		if err := os.WriteFile(filepath.Join(dir, "audit-report.md"), []byte(auditReport), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if acsVerdict != "" {
		if err := os.WriteFile(filepath.Join(dir, "acs-verdict.json"), []byte(acsVerdict), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestC1855_002_SentinelLessAuditReportIsAuditRanWithEmptyVerdict(t *testing.T) {
	dir := writeWorkspace(t, "## Verdict\nPASS in prose, no machine sentinel\n", `{"verdict":"PASS","red_count":0}`)
	audit, acs, ran := coherence.ReadCycleVerdicts(dir)
	if !ran || audit != "" || acs != "PASS" {
		t.Errorf("ReadCycleVerdicts = audit=%q acs=%q ran=%v; want \"\"/PASS/true", audit, acs, ran)
	}
}

func TestC1855_003_SentinelLessAuditReportNeverCountsAsForgedVerdict(t *testing.T) {
	dir := writeWorkspace(t, "## Verdict\nPASS in prose, no machine sentinel\n", `{"verdict":"PASS","red_count":0}`)
	audit, acs, ran := coherence.ReadCycleVerdicts(dir)
	for _, recorded := range []string{"FAIL", "WARN"} {
		got := coherence.CheckVerdictCoherence(coherence.VerdictInputs{Recorded: recorded, Audit: audit, ACS: acs, AuditRan: ran})
		if got.Incoherent || got.Reconciled {
			t.Errorf("recorded=%s with sentinel-less audit = %+v; want coherent zero value", recorded, got)
		}
	}
}

func TestC1855_004_SentinelBearingPassStillHaltsAsForgery(t *testing.T) {
	report := "<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"
	dir := writeWorkspace(t, report, `{"verdict":"PASS","red_count":0}`)
	audit, acs, ran := coherence.ReadCycleVerdicts(dir)
	got := coherence.CheckVerdictCoherence(coherence.VerdictInputs{Recorded: "FAIL", Audit: audit, ACS: acs, AuditRan: ran})
	if !got.Incoherent || got.Category != "verdict-incoherence" {
		t.Errorf("sentinel-bearing PASS vs recorded FAIL = %+v; want incoherent verdict-incoherence", got)
	}
}
