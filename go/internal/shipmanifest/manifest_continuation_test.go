package shipmanifest

// manifest_continuation_test.go — ADR-0076 slice C amendment #2 (architect
// review): a continuation cycle re-exposes the PRIOR attempt's files at ship
// time (post-build soft-reset to the original base), but this cycle's build
// report only declares what the resuming builder re-touched. Under
// manifest_gate=enforce that fails closed on resumed-but-undeclared paths, so
// Declared must UNION the prior attempt's declared manifest, located
// via the continuation manifest the adoption seam copies into this cycle's
// workspace.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func TestDeclaredManifest_UnionsPriorAttemptViaContinuation(t *testing.T) {
	runs := t.TempDir()
	priorWS := filepath.Join(runs, "cycle-91")
	curWS := filepath.Join(runs, "cycle-95")
	for _, d := range []string{priorWS, curWS} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Prior attempt declared prior_pkg/feature.go; this cycle declares only
	// what the resuming builder re-touched.
	if err := os.WriteFile(filepath.Join(priorWS, "build-report.md"), []byte("changed: prior_pkg/feature.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(curWS, "build-report.md"), []byte("changed: cur_pkg/fix.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteManifest(curWS, continuation.Continuation{SnapshotSHA: "abc", Cycle: 91}); err != nil {
		t.Fatal(err)
	}

	got := Declared(curWS, reportFiles)
	want := map[string]bool{}
	for _, p := range got {
		want[p] = true
	}
	if !want["cur_pkg/fix.go"] {
		t.Errorf("own declaration missing: %v", got)
	}
	if !want["prior_pkg/feature.go"] {
		t.Errorf("prior attempt's declaration must be unioned via the continuation manifest: %v", got)
	}
}

func TestDeclaredManifest_NoContinuationIsByteIdentical(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("changed: a/b.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Declared(ws, reportFiles)
	if len(got) != 1 || got[0] != "a/b.go" {
		t.Errorf("no continuation ⇒ unchanged behavior, got %v", got)
	}
}

var reportFiles = []string{"build-report.md", "test-report.md"}

func TestDeclared_ReadsOnlyTheNamedReports(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("changed: a/b.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte("changed: c/d.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Declared(ws, reportFiles); len(got) != 1 || got[0] != "a/b.go" {
		t.Errorf("Declared(%v) = %v, want [a/b.go] — a report the caller did not name must not widen the manifest", reportFiles, got)
	}
	if got := Declared(ws, nil); len(got) != 0 {
		t.Errorf("Declared(no reports) = %v, want empty — no manifest must stay distinguishable so the caller skips reconciliation", got)
	}
}
