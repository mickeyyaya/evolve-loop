package auditcalibration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPICoverNamed(t *testing.T) {
	root := t.TempDir()
	dossiers := filepath.Join(root, "dossiers")
	runs := filepath.Join(root, "runs")
	for _, dir := range []string{dossiers, runs} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	report, err := Generate(dossiers, runs)
	if err != nil || !strings.Contains(string(report), "Valid pairs: 0") {
		t.Fatalf("Generate(empty corpus) = %q, %v", report, err)
	}
}
