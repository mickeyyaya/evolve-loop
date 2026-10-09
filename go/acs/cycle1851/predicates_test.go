//go:build acs

package cycle1851

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecoherence"
)

const (
	artifactTreeSHA = "artifactsha111"
	ledgerTreeSHA   = "ledgersha222"
	provenanceCycle = 1851
)

func buildArtifactWithTreeSHA(treeSHA string) string {
	return "<!-- evolve:provenance phase=build cycle=1851 tree_sha=" + treeSHA + " -->\n# Build Report\n"
}

func ledgerEntry(role, treeSHA string) string {
	return `{"cycle":1851,"role":"` + role + `","tree_state_sha":"` + treeSHA + `"}`
}

func checkAgainstLedger(t *testing.T, ledgerLines []string, artifact, phase string) ([]phasecoherence.Violation, error) {
	t.Helper()
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := os.WriteFile(ledger, []byte(strings.Join(ledgerLines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	t.Setenv("EVOLVE_LEDGER_OVERRIDE", ledger)
	return phasecoherence.CheckProvenance(artifact, phasecoherence.ProvenanceFields{Phase: phase, Cycle: provenanceCycle})
}

func hasLedgerMismatch(violations []phasecoherence.Violation) bool {
	for _, v := range violations {
		if v.Kind == "provenance-mismatch" && strings.Contains(v.Message, "against ledger") {
			return true
		}
	}
	return false
}

func malformedLedgerViolations(violations []phasecoherence.Violation) []phasecoherence.Violation {
	var found []phasecoherence.Violation
	for _, v := range violations {
		if strings.Contains(strings.ToLower(v.Kind+" "+v.Message), "malformed") {
			found = append(found, v)
		}
	}
	return found
}

func TestC1851_001_LedgerLineOver64KiBIsScannedNotTruncated(t *testing.T) {
	oversizeRecordBeforeEntry := `{"cycle":1,"role":"scout","pad":"` + strings.Repeat("x", 100*1024) + `"}`
	violations, err := checkAgainstLedger(t,
		[]string{oversizeRecordBeforeEntry, ledgerEntry("builder", ledgerTreeSHA)},
		buildArtifactWithTreeSHA(artifactTreeSHA), "build")
	if err != nil {
		t.Fatalf("a valid ledger record over 64 KiB must be scanned, got error: %v", err)
	}
	if !hasLedgerMismatch(violations) {
		t.Errorf("the entry after a >64 KiB line was never cross-checked (silent partial scan): %+v", violations)
	}
}

func TestC1851_002_MalformedLedgerLineIsReportedAsWarn(t *testing.T) {
	violations, err := checkAgainstLedger(t,
		[]string{"{not json", ledgerEntry("builder", artifactTreeSHA)},
		buildArtifactWithTreeSHA(artifactTreeSHA), "build")
	if err != nil {
		t.Fatalf("a malformed line must be reported as a violation, not abort the check: %v", err)
	}
	malformed := malformedLedgerViolations(violations)
	if len(malformed) != 1 {
		t.Fatalf("want exactly 1 malformed-ledger violation, got %d: %+v", len(malformed), violations)
	}
	if malformed[0].Severity != "WARN" {
		t.Errorf("malformed-ledger severity = %q, want WARN", malformed[0].Severity)
	}
}

func TestC1851_003_CleanLedgerReportsNoMalformedLines(t *testing.T) {
	violations, err := checkAgainstLedger(t,
		[]string{ledgerEntry("scout", "x"), ledgerEntry("builder", artifactTreeSHA)},
		buildArtifactWithTreeSHA(artifactTreeSHA), "build")
	if err != nil {
		t.Fatalf("clean ledger: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("clean, consistent ledger must yield no violations, got %+v", violations)
	}
}

func TestC1851_004_UnreadableLedgerIsReportedNotSilent(t *testing.T) {
	ledgerIsADirectory := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	t.Setenv("EVOLVE_LEDGER_OVERRIDE", ledgerIsADirectory)
	violations, err := phasecoherence.CheckProvenance(buildArtifactWithTreeSHA(artifactTreeSHA),
		phasecoherence.ProvenanceFields{Phase: "build", Cycle: provenanceCycle})
	if err == nil && len(violations) == 0 {
		t.Errorf("an unreadable ledger produced neither an error nor a violation")
	}
}

func TestC1851_005_LedgerRoleMatchIsCaseInsensitive(t *testing.T) {
	cases := []struct {
		ledgerRole string
		phase      string
	}{
		{"Build", "build"},
		{"BUILDER", "build"},
		{"Audit", "auditor"},
		{"AUDITOR", "audit"},
		{"Scout", "scout"},
	}
	for _, tc := range cases {
		t.Run(tc.ledgerRole+"_vs_"+tc.phase, func(t *testing.T) {
			artifact := strings.Replace(buildArtifactWithTreeSHA(artifactTreeSHA), "phase=build", "phase="+tc.phase, 1)
			violations, err := checkAgainstLedger(t, []string{ledgerEntry(tc.ledgerRole, ledgerTreeSHA)}, artifact, tc.phase)
			if err != nil {
				t.Fatal(err)
			}
			if !hasLedgerMismatch(violations) {
				t.Errorf("ledger role %q did not match phase %q: case-sensitive alias lookup, got %+v", tc.ledgerRole, tc.phase, violations)
			}
		})
	}
}

func TestC1851_006_DifferentRoleStillDoesNotMatch(t *testing.T) {
	violations, err := checkAgainstLedger(t, []string{ledgerEntry("Scout", ledgerTreeSHA)},
		buildArtifactWithTreeSHA(artifactTreeSHA), "build")
	if err != nil {
		t.Fatal(err)
	}
	if hasLedgerMismatch(violations) {
		t.Errorf("a scout ledger entry must not be cross-checked against a build artifact: %+v", violations)
	}
}
