package research

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func recallCorpus(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		body := fmt.Sprintf(`- id: recall-%02d
  pattern: cycle-mid-execution-fail
  description: contract gate block in the build phase left the worktree predicate unsatisfied
  confidence: %.2f
  source: fixture
  type: failure-lesson
  category: episodic
  preventiveAction: re-dispatch the build with the contract escalation overlay
  failureContext:
    failedStep: build
    errorCategory: cycle-mid-execution-fail
    auditVerdict: FAIL
`, i, 0.9-float64(i)*0.05)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("recall-%02d.yaml", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func recallTestQuery() Query {
	return Query{
		Source:      "build",
		FailureMode: "contract gate block",
		Consequence: "cycle-mid-execution-fail",
		Keywords:    []string{"worktree", "predicate"},
	}
}

func lookupIDs(t *testing.T, kb KB) []string {
	t.Helper()
	got, err := kb.Lookup(context.Background(), recallTestQuery())
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	out := make([]string, 0, len(got))
	for _, l := range got {
		out = append(out, l.ID)
	}
	return out
}

func TestFileKB_ConfiguredRecallTakesRankingPrefix(t *testing.T) {
	dir := recallCorpus(t, 7)

	full := lookupIDs(t, NewFileKB([]string{dir}))
	if len(full) != 5 {
		t.Fatalf("fixture broken: default recall returned %d lessons over a 7-match corpus, want 5", len(full))
	}

	bounded := lookupIDs(t, NewFileKBWithRecall([]string{dir}, 3))
	if len(bounded) != 3 {
		t.Fatalf("recall=3 returned %d lessons, want 3", len(bounded))
	}
	for i := range bounded {
		if bounded[i] != full[i] {
			t.Errorf("bounded[%d] = %q, want %q — the bound must be a top-k prefix of the same ranking", i, bounded[i], full[i])
		}
	}
}

func TestFileKB_DefaultConstructorRecallUnchanged(t *testing.T) {
	dir := recallCorpus(t, 7)
	if got := lookupIDs(t, NewFileKB([]string{dir})); len(got) != 5 {
		t.Errorf("NewFileKB Lookup returned %d lessons, want 5 (existing callers must see no behaviour change)", len(got))
	}
}

func TestFileKB_NonPositiveRecallFallsBackToDefault(t *testing.T) {
	dir := recallCorpus(t, 7)
	for _, k := range []int{0, -1} {
		if got := lookupIDs(t, NewFileKBWithRecall([]string{dir}, k)); len(got) != 5 {
			t.Errorf("NewFileKBWithRecall(%d) returned %d lessons, want the default 5", k, len(got))
		}
	}
}

func TestFileKB_RecallAboveCorpusSizeReturnsAll(t *testing.T) {
	dir := recallCorpus(t, 3)
	if got := lookupIDs(t, NewFileKBWithRecall([]string{dir}, 25)); len(got) != 3 {
		t.Errorf("recall=25 over a 3-lesson corpus returned %d lessons, want 3", len(got))
	}
}

func TestFileKB_ZeroValueRecallStillRecalls(t *testing.T) {
	dir := recallCorpus(t, 7)
	if got := lookupIDs(t, &FileKB{roots: []string{dir}}); len(got) != 5 {
		t.Errorf("zero-value FileKB returned %d lessons, want the default 5", len(got))
	}
}
