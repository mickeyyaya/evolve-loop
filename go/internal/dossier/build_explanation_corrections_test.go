package dossier

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuild_RecordsTheExplanationCorrectionsOfAPassingCycle(t *testing.T) {
	t.Parallel()

	reviews := []string{"docs/explain/builds/cycle-7-r.md:8 states 9 lines; the scanner measures 8"}
	d, err := Build(7, BuildOpts{WorkspacePath: t.TempDir(), Goal: "x2", ExplanationCorrections: reviews})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"explanation_corrections":["docs/explain/builds/cycle-7-r.md:8 states 9 lines; the scanner measures 8"]`) {
		t.Fatalf("the dossier does not record the correction:\n%s", raw)
	}
	plain, _ := Build(7, BuildOpts{WorkspacePath: t.TempDir(), Goal: "x2"})
	if raw, _ := json.Marshal(plain); strings.Contains(string(raw), "explanation_corrections") {
		t.Errorf("a cycle with no correction records an empty one:\n%s", raw)
	}
}
