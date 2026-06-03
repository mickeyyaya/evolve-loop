package research

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// writeLesson writes a minimal valid lesson YAML file into dir.
func writeLesson(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

const auditEgpsLesson = `- id: inst-L001
  pattern: egps-red-without-fix
  description: "Audit failed because EGPS predicates were red and the builder shipped anyway."
  confidence: 0.9
  type: failure-lesson
  preventiveAction: "Always run the predicate suite before declaring PASS."
  failureContext:
    failedStep: audit
    errorCategory: integration
    auditVerdict: FAIL
`

const buildCompileLesson = `- id: inst-L002
  pattern: compile-fail-untested
  description: "Build produced code that did not compile under race detector."
  confidence: 0.6
  type: failure-lesson
  preventiveAction: "Run go build with race before handoff."
  failureContext:
    failedStep: build
    errorCategory: tool-use
    auditVerdict: FAIL
`

func TestFileKBLookup(t *testing.T) {
	dir := t.TempDir()
	writeLesson(t, dir, "audit.yaml", auditEgpsLesson)
	writeLesson(t, dir, "build.yaml", buildCompileLesson)

	kb := NewFileKB([]string{dir})

	t.Run("step match ranks the matching-source lesson first", func(t *testing.T) {
		got, err := kb.Lookup(context.Background(), Query{Source: "audit", Keywords: []string{"predicate"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 {
			t.Fatal("expected at least one lesson")
		}
		if got[0].ID != "inst-L001" {
			t.Errorf("top lesson = %q, want inst-L001 (audit step match)", got[0].ID)
		}
	})

	t.Run("keyword-only match still returns a lesson", func(t *testing.T) {
		got, err := kb.Lookup(context.Background(), Query{FailureMode: "compile under race"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID != "inst-L002" {
			t.Fatalf("got %+v, want only inst-L002", ids(got))
		}
	})

	t.Run("no match yields empty, not error (novel-failure signal)", func(t *testing.T) {
		got, err := kb.Lookup(context.Background(), Query{Source: "scout", FailureMode: "zzqqxx-nonsense"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("expected no matches, got %v", ids(got))
		}
	})
}

func TestFileKBHigherConfidenceWinsTie(t *testing.T) {
	dir := t.TempDir()
	// Two lessons with identical textual overlap on the keyword "predicate"
	// but different confidence; the higher-confidence one must rank first.
	writeLesson(t, dir, "a.yaml", `- id: inst-A
  pattern: p
  description: "predicate predicate"
  confidence: 0.4
  failureContext: {failedStep: audit}
`)
	writeLesson(t, dir, "b.yaml", `- id: inst-B
  pattern: p
  description: "predicate predicate"
  confidence: 0.95
  failureContext: {failedStep: audit}
`)
	kb := NewFileKB([]string{dir})
	got, err := kb.Lookup(context.Background(), Query{Keywords: []string{"predicate"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "inst-B" {
		t.Fatalf("got %v, want inst-B ranked first (higher confidence)", ids(got))
	}
}

func TestFileKBDedupesByIDAcrossRoots(t *testing.T) {
	r1, r2 := t.TempDir(), t.TempDir()
	writeLesson(t, r1, "a.yaml", auditEgpsLesson)
	writeLesson(t, r2, "a-copy.yaml", auditEgpsLesson) // same id inst-L001
	kb := NewFileKB([]string{r1, r2})
	got, err := kb.Lookup(context.Background(), Query{Source: "audit"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 deduped lesson, got %d: %v", len(got), ids(got))
	}
}

func TestFileKBMissingRootIsEmptyNotError(t *testing.T) {
	kb := NewFileKB([]string{"/no/such/dir"})
	got, err := kb.Lookup(context.Background(), Query{Source: "audit"})
	if err != nil {
		t.Fatalf("missing root should not error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", ids(got))
	}
}

func TestFileKBSkipsMalformedFile(t *testing.T) {
	dir := t.TempDir()
	writeLesson(t, dir, "good.yaml", auditEgpsLesson)
	writeLesson(t, dir, "bad.yaml", "this: is: not: valid: yaml: [")
	kb := NewFileKB([]string{dir})
	got, err := kb.Lookup(context.Background(), Query{Source: "audit"})
	if err != nil {
		t.Fatalf("malformed file should be skipped, not error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "inst-L001" {
		t.Fatalf("got %v, want only the well-formed inst-L001", ids(got))
	}
}

func TestSplitSearchPaths(t *testing.T) {
	got := SplitSearchPaths("a/:: b/ :c/")
	want := []string{"a/", "b/", "c/"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("path[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLessonDigestIsOneLine(t *testing.T) {
	l := Lesson{ID: "inst-L001", Pattern: "egps-red", PreventiveAction: "Run the suite. Then ship."}
	d := l.Digest()
	want := "inst-L001 (egps-red): Run the suite"
	if d != want {
		t.Errorf("Digest() = %q, want %q", d, want)
	}
}

func ids(ls []Lesson) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.ID
	}
	return out
}
