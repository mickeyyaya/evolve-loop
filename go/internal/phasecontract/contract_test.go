package phasecontract

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSectionPresent(t *testing.T) {
	s := Section{Canonical: "## Changes", Accepted: []string{"## Changes", "## Files Modified"}}
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"canonical present", "intro\n## Changes\n- x", true},
		{"legacy variant present", "## Files Modified\n- x", true},
		{"none present", "## Summary\n- x", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		if got := s.Present(c.content); got != c.want {
			t.Errorf("%s: Present()=%v want %v", c.name, got, c.want)
		}
	}
}

func TestReportComplete(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"both groups present", "## AC-Materialization\n## RED Run Output\n", true},
		{"both via legacy variants", "## Acceptance\n## RED Tests\n", true},
		{"only acceptance group", "## Coverage Map\n", false},
		{"only red group", "## Test Files Written\n", false},
		{"neither", "## Notes\n", false},
	}
	for _, c := range cases {
		if got := TDD.Complete(c.content); got != c.want {
			t.Errorf("%s: TDD.Complete()=%v want %v", c.name, got, c.want)
		}
	}
}

func TestReportComplete_NoSectionsTriviallyComplete(t *testing.T) {
	if !(Report{Phase: "x"}).Complete("anything") {
		t.Error("a Report with no sections must be trivially complete")
	}
}

func TestProducersDeclareCanonical(t *testing.T) {
	agentsDir := agentsDir(t)
	for _, r := range All {
		r := r
		t.Run(r.Phase, func(t *testing.T) {
			if len(r.Producers) == 0 {
				t.Fatalf("phase %q declares no producers", r.Phase)
			}
			if len(r.Sections) == 0 {
				t.Fatalf("phase %q declares no sections", r.Phase)
			}
			union := producerUnion(t, agentsDir, r.Producers)
			for _, s := range r.Sections {
				if !strings.Contains(union, s.Canonical) {
					t.Errorf("phase %q: canonical heading %q is not declared by any producer %v — "+
						"template/contract drift. Reconcile contract.go with agents/.",
						r.Phase, s.Canonical, r.Producers)
				}
			}
		})
	}
}

func agentsDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate agents/")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "agents")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("agents/ not found at %s: %v", dir, err)
	}
	return dir
}

func producerUnion(t *testing.T, agentsDir string, producers []string) string {
	t.Helper()
	var b strings.Builder
	for _, p := range producers {
		data, err := os.ReadFile(filepath.Join(agentsDir, p+".md"))
		if err != nil {
			t.Fatalf("read producer %q: %v", p, err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String()
}
