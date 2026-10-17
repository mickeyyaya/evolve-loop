//go:build acs

package cycle1924

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const researchIndexRel = "docs/research-index.md"

type restoredRow struct {
	link  string
	facts []string
}

var lossyRows = []restoredRow{
	{"research/cli-coverage-inventory-2026-09-30.md", []string{"147"}},
	{"reports/inbox-review-2026-09-30.md", []string{"six"}},
	{"reports/comment-strip-clean-code-review-2026-09-30.md", []string{"defects"}},
	{"architecture/adr/0111-code-carries-no-comments.md", []string{"design notes"}},
	{"architecture/adr/0114-the-acs-verdict-is-always-written-and-complete.md", []string{"scrubbed"}},
	{"architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md", []string{"pure-docs", "added-comment rule"}},
	{"architecture/adr/0116-inbox-curation-verbs.md", []string{"verb-owned", "lifecycle stamp"}},
	{"architecture/adr/0117-pre-handoff-probes-run-the-build-floor.md", []string{"1763", "1764", "1788"}},
	{"research/review-convergence-limited-rounds-2026-10.md", []string{"VRR-Stop", "OpenCodeReview", "33,596", "CR1"}},
	{"plans/convergence-policy-2026-10.md", []string{"CQ1", "V3b", "rung 2"}},
	{"architecture/adr/0126-every-iterative-loop-converges-or-escalates.md", []string{"never deferred", "J_0"}},
	{"research/audit-evaluation-contract-prior-art-2026-10.md", []string{"StrongDM", "SpecBench", "AR1"}},
	{"architecture/audit-evaluation-contract.md", []string{"digests", "ADR-0125"}},
	{"plans/audit-evaluation-contract-2026-10.md", []string{"AR1", "flip"}},
	{"architecture/adr/0125-audit-publishes-its-evaluation-contract.md", []string{"probe instances"}},
	{"architecture/adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md", []string{"resume"}},
}

func indexRow(t *testing.T, link string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(researchIndexRel)))
	if err != nil {
		t.Fatal(err)
	}
	var matches []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "| ["+link+"](") {
			matches = append(matches, line)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("want exactly one index row for %s, got %d", link, len(matches))
	}
	return matches[0]
}

func recordCell(row string) string {
	cells := strings.Split(row, " | ")
	return strings.ToLower(cells[len(cells)-1])
}

func TestC1924_001_LossyRowsRestoreDroppedFacts(t *testing.T) {
	for _, r := range lossyRows {
		r := r
		t.Run(filepath.Base(r.link), func(t *testing.T) {
			cell := recordCell(indexRow(t, r.link))
			for _, fact := range r.facts {
				if !strings.Contains(cell, strings.ToLower(fact)) {
					t.Errorf("row %s lost the fact %q: %s", r.link, fact, cell)
				}
			}
		})
	}
}

func TestC1924_002_ADR0115RowNamesTheAddedCommentExemption(t *testing.T) {
	cell := recordCell(indexRow(t, "architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md"))
	if !strings.Contains(cell, "docs/") || !strings.Contains(cell, "added-comment rule") {
		t.Errorf("ADR-0115 row must say docs/ is exempt from the added-comment rule: %s", cell)
	}
	for _, ambiguous := range []string{"exempt from the rule", "exempt from this rule"} {
		if strings.Contains(cell, ambiguous) {
			t.Errorf("ADR-0115 row keeps the ambiguous phrase %q: %s", ambiguous, cell)
		}
	}
}

func TestC1924_003_ResearchIndexPassesStrictSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, researchIndexRel)
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, out, errOut)
	}
}

func TestC1924_004_IndexKeepsEveryRowItLinked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", "origin/main:"+researchIndexRel)
	if err != nil || code != 0 {
		t.Fatalf("cannot read base index: rc=%d err=%v", code, err)
	}
	head, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(researchIndexRel)))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "| [") {
			continue
		}
		link := line[:strings.Index(line, "](")+2]
		if !strings.Contains(string(head), link) {
			t.Errorf("index dropped the row %s", link)
		}
	}
}
