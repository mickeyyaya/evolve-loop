package evalgate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSlugParserContract pins SelectedSlugs to the scout report format: every
// token the parser greps for must still be declared by the scout producer
// templates. This matters because Gate A (materialization) BLOCKS at enforce
// based on the slugs this parser extracts — so if the scout template renamed
// "## Selected Tasks" or the "- **Slug:**" bullet, enforce-mode could hard-block
// a HEALTHY cycle with no alarm. This is the slug-extraction analogue of
// phasecontract's TestProducersDeclareCanonical (the verdict-heading drift
// alarm): it closes the asymmetry where the verdict-heading proxy got a contract
// test but the slug-extraction proxy did not.
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

// scoutProducerUnion returns the concatenated text of the scout agent templates,
// located from this test file's path (robust to cwd). evalgate lives at
// go/internal/evalgate/, so agents/ is three levels up.
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
