package cycleclassify

import (
	"os"
	"path/filepath"
	"testing"
)

// TestClassify_HangClassifier_ReclassifiesSHIPPED covers Gap #6: when
// EVOLVE_HANG_CLASSIFIER=1, a SHIPPED-verdict report + matching git
// log entry should reclassify integrity-breach as exit-transport-hang.
func TestClassify_HangClassifier_ReclassifiesSHIPPED(t *testing.T) {
	// NOT t.Parallel — mutates package-level gitLogFn + env.
	t.Setenv("EVOLVE_HANG_CLASSIFIER", "1")
	prev := gitLogFn
	defer func() { gitLogFn = prev }()
	gitLogFn = func(cycleNum string) bool {
		return cycleNum == "42"
	}

	parent := t.TempDir()
	ws := filepath.Join(parent, "cycle-42")
	_ = os.MkdirAll(ws, 0o755)
	report := `# Cycle 42 orchestrator report

Some prelude content.

## Verdict
SHIPPED
`
	_ = os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(report), 0o644)

	r := Classify(ws)
	if r.Class != ClassExitTransportHang {
		t.Fatalf("got %q want exit-transport-hang (marker=%q)", r.Class, r.Marker)
	}
}

func TestClassify_HangClassifier_NoCommitFalsePositive(t *testing.T) {
	t.Setenv("EVOLVE_HANG_CLASSIFIER", "1")
	prev := gitLogFn
	defer func() { gitLogFn = prev }()
	gitLogFn = func(string) bool { return false } // no matching commit

	parent := t.TempDir()
	ws := filepath.Join(parent, "cycle-43")
	_ = os.MkdirAll(ws, 0o755)
	report := "## Verdict\nSHIPPED\n"
	_ = os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(report), 0o644)

	r := Classify(ws)
	// Without git commit, SHIPPED alone doesn't reclassify — falls to
	// integrity-breach (no other markers).
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("got %q want integrity-breach (no commit → no reclassify)", r.Class)
	}
}

func TestClassify_HangClassifier_DisabledByDefault(t *testing.T) {
	t.Setenv("EVOLVE_HANG_CLASSIFIER", "") // explicitly off
	prev := gitLogFn
	defer func() { gitLogFn = prev }()
	gitLogFn = func(string) bool { return true } // would match if checked

	parent := t.TempDir()
	ws := filepath.Join(parent, "cycle-44")
	_ = os.MkdirAll(ws, 0o755)
	report := "## Verdict\nSHIPPED\n"
	_ = os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(report), 0o644)

	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("got %q want integrity-breach (env disabled)", r.Class)
	}
}

func TestClassify_HangClassifier_NonShippedNoReclassify(t *testing.T) {
	t.Setenv("EVOLVE_HANG_CLASSIFIER", "1")
	prev := gitLogFn
	defer func() { gitLogFn = prev }()
	gitLogFn = func(string) bool { return true }

	parent := t.TempDir()
	ws := filepath.Join(parent, "cycle-45")
	_ = os.MkdirAll(ws, 0o755)
	// Report has Verdict block but says FAILED on the SAME line so
	// audit-fail regex (line-by-line) matches first. Confirms hang
	// classifier doesn't override stronger classifications.
	report := "Verdict: FAILED — auditor rejected\n"
	_ = os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(report), 0o644)

	r := Classify(ws)
	// FAILED → audit-fail regex matches first (Verdict.*FAIL).
	if r.Class != ClassAuditFail {
		t.Fatalf("got %q want audit-fail (not exit-transport-hang)", r.Class)
	}
}

func TestClassify_HangClassifier_BadWorkspacePath(t *testing.T) {
	t.Setenv("EVOLVE_HANG_CLASSIFIER", "1")
	prev := gitLogFn
	defer func() { gitLogFn = prev }()
	gitLogFn = func(string) bool { return true }

	// Workspace name doesn't follow cycle-N convention.
	parent := t.TempDir()
	ws := filepath.Join(parent, "not-a-cycle-dir")
	_ = os.MkdirAll(ws, 0o755)
	report := "## Verdict\nSHIPPED\n"
	_ = os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(report), 0o644)

	r := Classify(ws)
	if r.Class == ClassExitTransportHang {
		t.Fatalf("should NOT reclassify when workspace path lacks cycle-N suffix")
	}
}

func TestShippedAfterVerdict(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"shipped on next line", "## Verdict\nSHIPPED\n", true},
		{"shipped after blank line", "## Verdict\n\n\nshipped (lowercase)\n", true},
		{"no verdict section", "no markers here", false},
		{"verdict but no SHIPPED", "## Verdict\nFAIL\n", false},
		{"shipped before verdict ignored", "shipped\n## Verdict\nFAIL\n", false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shippedAfterVerdict([]byte(tc.body)); got != tc.want {
				t.Fatalf("shippedAfterVerdict(%q)=%v want %v", tc.body, got, tc.want)
			}
		})
	}
}
