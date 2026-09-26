package evalgate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSlugParserContract(t *testing.T) {
	union := scoutProducerUnion(t)
	for _, tok := range []string{
		"## Selected Tasks", // selectedTaskSlugs section heading
		"**Slug:**",         // slugLineRE bullet
		"## Decision Trace", // decisionTraceSelected fenced-block heading
		"finalDecision",     // decision-trace JSON field
		"selected",          // the finalDecision value the parser matches
	} {
		if !strings.Contains(union, tok) {
			t.Errorf("scout producer templates no longer declare %q — SelectedSlugs (Gate A's block input) "+
				"may silently mis-extract; reconcile slugs.go with agents/evolve-scout*.md", tok)
		}
	}
}

// scoutProducerUnion reads agents/evolve-scout*.md relative to this file, so it
// does not depend on the working directory.
func scoutProducerUnion(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate agents/")
	}
	agentsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "agents")
	var b strings.Builder
	for _, name := range []string{"evolve-scout.md", "evolve-scout-reference.md"} {
		data, err := os.ReadFile(filepath.Join(agentsDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String()
}
