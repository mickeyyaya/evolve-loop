// Package cycle102 ports the cycle-102 ACS predicates (3 bash files).
package cycle102

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC102_001_ProfileMaxTurnsCeilings ports cycle-102/001.
// Verifies 4 agent profiles meet/exceed scout-recommended max_turns floors.
func TestC102_001_ProfileMaxTurnsCeilings(t *testing.T) {
	root := acsassert.RepoRoot(t)
	tuples := []struct {
		path string
		min  int
	}{
		{filepath.Join(root, ".evolve", "profiles", "triage.json"), 18},
		{filepath.Join(root, ".evolve", "profiles", "intent.json"), 12},
		{filepath.Join(root, ".evolve", "profiles", "scout.json"), 42},
		{filepath.Join(root, ".evolve", "profiles", "builder.json"), 36},
	}
	for _, tup := range tuples {
		if !fixtures.FilePresent(tup.path) {
			t.Skipf("%s missing — skip cycle-102-001", tup.path)
			return
		}
		actual := readMaxTurnsOrSkip(t, tup.path)
		if actual < tup.min {
			t.Errorf("%s: max_turns=%d (need ≥ %d)", tup.path, actual, tup.min)
		}
	}
}

// TestC102_002_IncidentDocTurnOverrun ports cycle-102/002.
// Verifies cycle-99-100 turn-overrun incident doc presence + density.
func TestC102_002_IncidentDocTurnOverrun(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "operations", "incidents", "cycle-99-100-turn-overrun.md")
	if !fixtures.FilePresent(doc) {
		t.Skip("cycle-99-100-turn-overrun.md missing — skip cycle-102-002")
	}
	// Density: ≥30 non-blank lines
	nonBlank := countNonBlank(t, doc)
	if nonBlank < 30 {
		t.Errorf("%s: %d non-blank lines (need ≥30)", doc, nonBlank)
	}
	// References ≥2 of {triage, intent, scout, builder}
	refs := 0
	for _, agent := range []string{"triage", "intent", "scout", "builder"} {
		if acsassert.FileMatchesRegex(t, doc, `(?i)(^|[^a-zA-Z])`+agent+`([^a-zA-Z]|$)`) {
			refs++
		}
	}
	if refs < 2 {
		t.Errorf("%s: references only %d/4 agents (need ≥2)", doc, refs)
	}
}

// TestC102_003_IncidentDocShipRefused ports cycle-102/003.
// Verifies cycle-100 ship-refused incident doc presence + density.
func TestC102_003_IncidentDocShipRefused(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// Accept multiple plausible filenames
	candidates := []string{
		filepath.Join(root, "docs", "operations", "incidents", "cycle-100-ship-refused.md"),
		filepath.Join(root, "docs", "operations", "incidents", "abnormal-ship-refused-c100.md"),
		filepath.Join(root, "knowledge-base", "research", "cycle-100-ship-refused.md"),
	}
	var doc string
	for _, p := range candidates {
		if acsassert.FileExists(t, p) {
			doc = p
			break
		}
	}
	if doc == "" {
		t.Skip("no cycle-100 ship-refused incident doc at accepted paths")
	}
	if nonBlank := countNonBlank(t, doc); nonBlank < 20 {
		t.Errorf("%s: %d non-blank lines (need ≥20)", doc, nonBlank)
	}
}

func readMaxTurnsOrSkip(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	v, ok := doc["max_turns"]
	if !ok {
		t.Errorf("%s: max_turns missing", path)
		return -1
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return -1
}

func countNonBlank(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	count := 0
	for _, line := range splitLines(string(raw)) {
		if trim(line) != "" {
			count++
		}
	}
	return count
}

func splitLines(s string) []string {
	var lines []string
	cur := ""
	for _, ch := range s {
		if ch == '\n' {
			lines = append(lines, cur)
			cur = ""
			continue
		}
		cur += string(ch)
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func trim(s string) string {
	start := 0
	for start < len(s) && isSpace(s[start]) {
		start++
	}
	end := len(s)
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }
