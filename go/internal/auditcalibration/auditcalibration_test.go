package auditcalibration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditCalibration_MalformedOrMissingPairIsExcluded(t *testing.T) {
	dossiers, runs := testCorpus(t)
	writeTestFile(t, filepath.Join(dossiers, "cycle-1.json"), testDossier(1, "PASS"))
	writeTestFile(t, filepath.Join(runs, "cycle-1", "marker"), "sample")
	writeTestFile(t, filepath.Join(dossiers, "cycle-2.json"), testDossier(2, "PASS"))
	writeTestFile(t, filepath.Join(runs, "cycle-2", "audit-chain-shadow.json"), "{not json")

	report, err := Generate(dossiers, runs)
	if err != nil {
		t.Fatal(err)
	}
	text := string(report)
	for _, want := range []string{"Valid pairs: 0", "Excluded: 2", "| 1 | missing-shadow |", "| 2 | malformed-shadow |"} {
		if !strings.Contains(text, want) {
			t.Errorf("report missing %q:\n%s", want, text)
		}
	}
}

func TestAuditCalibration_NarrativePassForceOverriddenToFail(t *testing.T) {
	dossiers, runs := testCorpus(t)
	writeTestFile(t, filepath.Join(dossiers, "cycle-7.json"), testDossier(7, "FAIL"))
	writeTestFile(t, filepath.Join(runs, "cycle-7", "audit-chain-shadow.json"), `{"cycle":7,"phase":"audit","narrative_verdict":"PASS","chain_verdict":"PASS","shipped_verdict":"FAIL","overrode_by":["EGPS"]}`)
	writeTestFile(t, filepath.Join(runs, "cycle-7", "audit-fail-reason.json"), `{"reasons":["verdict-conflict: forced fail"]}`)

	report, err := Generate(dossiers, runs)
	if err != nil {
		t.Fatal(err)
	}
	text := string(report)
	for _, want := range []string{"| PASS | FAIL | 1 |", "| 7 | PASS | FAIL | EGPS |", "| 7 | PASS | PASS | FAIL | FAIL | EGPS | verdict-conflict |"} {
		if !strings.Contains(text, want) {
			t.Errorf("report missing %q:\n%s", want, text)
		}
	}
}

func testCorpus(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dossiers := filepath.Join(root, "knowledge-base", "cycles")
	runs := filepath.Join(root, ".evolve", "runs")
	for _, dir := range []string{dossiers, runs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dossiers, runs
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testDossier(cycle int, verdict string) string {
	extra := ""
	if verdict == "FAIL" {
		extra = `,"defects":[{"id":"d","summary":"failed"}],"carryover":[{"id":"c","action":"fix"}]`
	}
	return fmt.Sprintf(`{"cycle":%d,"goal":"test","final_verdict":%q,"phases":[{"name":"audit","verdict":%q}]%s}`, cycle, verdict, verdict, extra)
}
