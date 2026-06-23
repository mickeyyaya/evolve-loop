//go:build acs

// Package cycle42 ports the cycle-42 ACS predicates (4 files) — all
// file-grep / file-existence style — from acs/cycle-42/*.sh.
package cycle42

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	stdout, _, code, err := acsassert.SubprocessOutput("git", "rev-parse", "--show-toplevel")
	if err != nil || code != 0 {
		t.Skipf("not in a git work tree: code=%d err=%v", code, err)
	}
	return strings.TrimSpace(stdout)
}

// roadmapPath is the file every cycle-42 predicate gates on.
func roadmapPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(repoRoot(t), "docs", "architecture", "token-reduction-roadmap.md")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("token-reduction-roadmap.md missing (expected during fresh clone): %v", err)
	}
	return p
}

// TestC42_003_PNew13Done — status table row for P-NEW-13 must contain
// 'DONE (cycle 42)' AND its field-table 'Target cycle' must not say '43+'.
func TestC42_003_PNew13Done(t *testing.T) {
	doc := roadmapPath(t)
	acsassert.FileMatchesRegex(t, doc, `P-NEW-13.*DONE \(cycle 42\)`)

	raw, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("read %s: %v", doc, err)
	}
	if strings.Contains(string(raw), "Target cycle") && strings.Contains(string(raw), "43+") {
		// Sniff for the specific anti-pattern: 'Target cycle' followed by 43+ near P-NEW-13.
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, "Target cycle") && strings.Contains(line, "43+") {
				t.Errorf("P-NEW-13 field table Target cycle still shows '43+' — should be DONE: %s", line)
			}
		}
	}
}

// TestC42_004_PNew16Done — status table row for P-NEW-16 must contain
// 'DONE (cycle 42)'.
func TestC42_004_PNew16Done(t *testing.T) {
	doc := roadmapPath(t)
	acsassert.FileMatchesRegex(t, doc, `P-NEW-16.*DONE \(cycle 42\)`)
}

// TestC42_005_P6CitationFix — PSMAS may not be cited with arXiv:2510.26585;
// 2604.17400 must appear somewhere.
func TestC42_005_P6CitationFix(t *testing.T) {
	doc := roadmapPath(t)
	raw, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("read %s: %v", doc, err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "PSMAS") && strings.Contains(line, "2510.26585") {
			t.Errorf("PSMAS still cited with arXiv:2510.26585 — found: %s", line)
		}
	}
	acsassert.FileContains(t, doc, "2604.17400")
}

// TestC42_006_PNew17Exists — P-NEW-17 section + status row + KB file.
func TestC42_006_PNew17Exists(t *testing.T) {
	root := repoRoot(t)
	doc := filepath.Join(root, "docs", "architecture", "token-reduction-roadmap.md")
	kb := filepath.Join(root, "knowledge-base", "research", "cache-ttl-march-2026-impact.md")

	if _, err := os.Stat(doc); err != nil {
		t.Skipf("roadmap missing: %v", err)
	}
	acsassert.FileContains(t, doc, "P-NEW-17")
	acsassert.FileMatchesRegex(t, doc, `P-NEW-17.*RESEARCH|P-NEW-17.*PENDING|P-NEW-17.*cycle 43`)
	acsassert.FileExists(t, kb)
}
