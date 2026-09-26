package cycleclassify

import (
	"os"
	"path/filepath"
	"testing"
)

// setHangClassifierForTest swaps the package-level hangClassifierFn, so callers cannot run in parallel.
func setHangClassifierForTest(t *testing.T, enabled bool) {
	t.Helper()
	prev := hangClassifierFn
	hangClassifierFn = func() bool { return enabled }
	t.Cleanup(func() { hangClassifierFn = prev })
}

func TestSetHangClassifier_WiresToggleFromPolicy(t *testing.T) {
	// Not parallel: mutates the package-level hangClassifierFn.
	prev := hangClassifierFn
	t.Cleanup(func() { hangClassifierFn = prev })

	SetHangClassifier(true)
	if !hangClassifierFn() {
		t.Error("SetHangClassifier(true): gate not enabled")
	}
	SetHangClassifier(false)
	if hangClassifierFn() {
		t.Error("SetHangClassifier(false): gate not disabled")
	}
}

func TestClassify_HangClassifier_ReclassifiesSHIPPED(t *testing.T) {
	// Not parallel: mutates the package-level gitLogFn and hangClassifierFn.
	setHangClassifierForTest(t, true)
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
	setHangClassifierForTest(t, true)
	prev := gitLogFn
	defer func() { gitLogFn = prev }()
	gitLogFn = func(string) bool { return false }

	parent := t.TempDir()
	ws := filepath.Join(parent, "cycle-43")
	_ = os.MkdirAll(ws, 0o755)
	report := "## Verdict\nSHIPPED\n"
	_ = os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(report), 0o644)

	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Fatalf("got %q want integrity-breach (no commit → no reclassify)", r.Class)
	}
}

func TestClassify_HangClassifier_DisabledByDefault(t *testing.T) {
	setHangClassifierForTest(t, false) // pins the disabled branch; the package default is also off
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
		t.Fatalf("got %q want integrity-breach (disabled)", r.Class)
	}
}

func TestClassify_HangClassifier_NonShippedNoReclassify(t *testing.T) {
	setHangClassifierForTest(t, true)
	prev := gitLogFn
	defer func() { gitLogFn = prev }()
	gitLogFn = func(string) bool { return true }

	parent := t.TempDir()
	ws := filepath.Join(parent, "cycle-45")
	_ = os.MkdirAll(ws, 0o755)
	// FAILED sits on the Verdict line itself so the line-by-line audit-fail regex matches first.
	report := "Verdict: FAILED — auditor rejected\n"
	_ = os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(report), 0o644)

	r := Classify(ws)
	if r.Class != ClassAuditFail {
		t.Fatalf("got %q want audit-fail (not exit-transport-hang)", r.Class)
	}
}

func TestClassify_HangClassifier_BadWorkspacePath(t *testing.T) {
	setHangClassifierForTest(t, true)
	prev := gitLogFn
	defer func() { gitLogFn = prev }()
	gitLogFn = func(string) bool { return true }

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
