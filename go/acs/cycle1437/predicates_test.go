//go:build acs

// Package cycle1437 materialises the cycle-1437 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//   - salvage-baseline-measured-writeup → §6 of
//     docs/research/deliverable-alignment-2026-08/README.md still summarises the
//     recoverable-malformed rate as "not yet instrumented", contradicting §7,
//     which already carries the audited measured table (cycle-1389). This cycle
//     replaces that stale clause with the measured figures + an evidence
//     citation, and adds a §6.3 entry per the doc's own §3.8 issue/gap/solution
//     convention.
//
// Predicate strategy — the deliverable of this task IS a document, so the
// "system under test" is the emitted artifact itself. To stay out of the
// cycle-85 degenerate-predicate class (a grep for a magic string the
// implementer can trivially paste), NO predicate here asserts a hardcoded
// sentence. Every assertion is either:
//
//   - CROSS-REFERENTIAL — the expected values are *derived at runtime* from §7's
//     committed measured table and then required to appear in §6, so §6 can only
//     go green by agreeing with the audited source of record (001, 002, 003);
//   - STRUCTURAL-BY-DERIVATION — §6.3's required subsection markers are derived
//     from §6.1's own committed structure, not from a literal list (004);
//   - a real FILESYSTEM check — the code path §6.3 cites must exist on disk (004);
//   - a NEGATIVE / anti-invention assertion — history in §7 must survive the edit
//     (001), and no count may appear in §6 that §7 does not license (005).
//
// Root resolution: everything is read under acsassert.RepoRoot(t) (the cycle
// worktree, where Builder writes). Deliberately NO read of
// .evolve/runs/cycle-1389/bad-verdict-baseline.jsonl — that main-plane read is
// exactly what false-RED'd cycle-1434 (wrong-project-root, fixed by #449); the
// citation is checked as a textual cross-reference to §7 instead, which is the
// actual acceptance criterion and is worktree-local.
package cycle1437

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// readmeRelPath is the single target file named by scout-report targetFiles and
// the triage decision.
const readmeRelPath = "docs/research/deliverable-alignment-2026-08/README.md"

// salvageCodeRelPath is the producing code §6.3 must point readers at (the
// classifier that generated §7's baseline).
const salvageCodeRelPath = "go/internal/deliverable/salvage_instrument.go"

// stalePlaceholder is the clause §6 must stop asserting. Whitespace-normalised
// before matching because the doc hard-wraps at ~80 columns, so the phrase
// straddles a newline today.
const stalePlaceholder = "not yet instrumented"

// readReadme returns the whole target document, failing loudly when absent.
func readReadme(t *testing.T) string {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), readmeRelPath)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("RED: cannot read target document %s: %v", readmeRelPath, err)
	}
	return string(b)
}

// section returns the body of the markdown section whose heading line begins
// with startPrefix, up to (exclusive) the next heading whose level is <= the
// start heading's level. ok is false when the heading is absent.
func section(doc, startPrefix string) (body string, ok bool) {
	lines := strings.Split(doc, "\n")
	startLevel := len(startPrefix) - len(strings.TrimLeft(startPrefix, "#"))
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, startPrefix) {
			start = i
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		ln := lines[i]
		if !strings.HasPrefix(ln, "#") {
			continue
		}
		lvl := len(ln) - len(strings.TrimLeft(ln, "#"))
		if lvl <= startLevel {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}

// normalize collapses all runs of whitespace to single spaces so assertions are
// immune to the document's 80-column hard wrapping.
func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

// section6Intro is §6's own prose — the heading through the first `###`
// subsection. This is the paragraph carrying the stale clause (the edit site);
// §6.1/§6.2 are landed history and are NOT in scope.
func section6Intro(t *testing.T, doc string) string {
	t.Helper()
	body, ok := section(doc, "## 6. ")
	if !ok {
		t.Fatalf("RED: §6 heading (\"## 6. \") not found in %s", readmeRelPath)
	}
	if idx := strings.Index(body, "\n### "); idx >= 0 {
		body = body[:idx]
	}
	return body
}

func section7(t *testing.T, doc string) string {
	t.Helper()
	body, ok := section(doc, "## 7. ")
	if !ok {
		t.Fatalf("RED: §7 heading (\"## 7. \") not found in %s", readmeRelPath)
	}
	return body
}

// measuredBaseline derives the audited figures from §7's committed table rather
// than hardcoding them, so a §6 edit can only pass by agreeing with §7. Returns
// (recoverable, total, percent) as they are literally written there.
func measuredBaseline(t *testing.T, sec7 string) (recoverable, total, percent string) {
	t.Helper()
	recRe := regexp.MustCompile(`classifier-\*\*recoverable\*\*\s*\|\s*\*\*(\d+)\s*\((\d+(?:\.\d+)?%)\)`)
	m := recRe.FindStringSubmatch(sec7)
	if m == nil {
		t.Fatalf("RED: §7 recoverable row not parseable — the measured table this cycle must cite is missing or reshaped")
	}
	totRe := regexp.MustCompile(`blocks \(baseline records written\)\s*\|\s*(\d+)`)
	mt := totRe.FindStringSubmatch(sec7)
	if mt == nil {
		t.Fatalf("RED: §7 total-blocks row not parseable — cannot derive the denominator")
	}
	return m[1], mt[1], m[2]
}

// TestC1437_001_Section6DropsStalePlaceholderWhileSection7KeepsHistory is the
// crux negative predicate. It has two halves and both must hold:
//
//	POSITIVE-BY-ABSENCE — §6's own prose no longer claims the rate is
//	uninstrumented (the actual defect).
//	ANTI-REGRESSION — §7's historical QUOTE of that old wording survives. §7
//	opens by quoting §6's stale sentence to explain what it replaced; a
//	document-wide find/replace (the obvious cheap "fix") would erase that
//	provenance. This half fails such an edit and passes a scoped one.
func TestC1437_001_Section6DropsStalePlaceholderWhileSection7KeepsHistory(t *testing.T) {
	doc := readReadme(t)
	intro := normalize(section6Intro(t, doc))
	sec7 := normalize(section7(t, doc))

	if strings.Contains(intro, stalePlaceholder) {
		t.Errorf("RED: §6 prose still asserts %q — the stale placeholder contradicting §7's measured table was not replaced", stalePlaceholder)
	}
	if !strings.Contains(sec7, stalePlaceholder) {
		t.Errorf("RED: §7 no longer quotes the historical %q wording — the edit was applied document-wide and destroyed §7's provenance record; scope the change to §6", stalePlaceholder)
	}
}

// TestC1437_002_Section6StatesMeasuredRateDerivedFromSection7 requires §6 to
// state the measured rate using the EXACT figures parsed out of §7's table at
// run time. Nothing is hardcoded: if §7 said other numbers, this predicate would
// demand those instead — so §6 cannot be greened with an invented rate, and the
// two sections cannot drift apart again.
func TestC1437_002_Section6StatesMeasuredRateDerivedFromSection7(t *testing.T) {
	doc := readReadme(t)
	intro := normalize(section6Intro(t, doc))
	recoverable, total, percent := measuredBaseline(t, normalize(section7(t, doc)))

	for _, want := range []struct{ label, token string }{
		{"recoverable count", recoverable},
		{"total bad_verdict blocks", total},
	} {
		re := regexp.MustCompile(`(^|[^0-9.])` + regexp.QuoteMeta(want.token) + `([^0-9.]|$)`)
		if !re.MatchString(intro) {
			t.Errorf("RED: §6 does not state the %s (%s) from §7's measured table", want.label, want.token)
		}
	}
	if !strings.Contains(intro, percent) {
		t.Errorf("RED: §6 does not state the measured recoverable rate %s from §7's table", percent)
	}
}

// TestC1437_003_Section6CitesEvidenceByPathMatchingSection7 requires the new §6
// text to carry an evidence citation, and requires that citation to be the SAME
// evidence path §7 cites — derived from §7, never hardcoded. A cross-reference
// to §7 itself also satisfies the "source of record" half, but a bare number
// with no provenance does not.
func TestC1437_003_Section6CitesEvidenceByPathMatchingSection7(t *testing.T) {
	doc := readReadme(t)
	intro := normalize(section6Intro(t, doc))
	sec7 := normalize(section7(t, doc))

	pathRe := regexp.MustCompile(`\.evolve/runs/cycle-\d+/bad-verdict-baseline\.jsonl`)
	evidence := pathRe.FindString(sec7)
	if evidence == "" {
		t.Fatalf("RED: §7 cites no bad-verdict-baseline.jsonl evidence path — cannot derive the citation §6 must match")
	}

	citesPath := strings.Contains(intro, evidence)
	citesSection := strings.Contains(intro, "§7")
	if !citesPath && !citesSection {
		t.Errorf("RED: §6's measured statement carries no provenance — cite the evidence path %s and/or cross-reference §7 as the source of record", evidence)
	}
	// The path form, when present, must be the exact one §7 uses: a stale or
	// invented cycle number would send readers at a file that never held these
	// counts.
	if p := pathRe.FindString(intro); p != "" && p != evidence {
		t.Errorf("RED: §6 cites evidence %s but §7's source of record is %s — the citations disagree", p, evidence)
	}
}

// TestC1437_004_Section63FollowsTemplateAndCitesLiveCode checks the new
// subsection. The required markers are DERIVED from §6.1's committed structure
// (the doc's §3.8 issue/gap/solution convention) rather than listed literally,
// and the code path §6.3 points at is verified to exist on disk AND be
// git-tracked — a dangling cross-reference is the failure mode this catches.
func TestC1437_004_Section63FollowsTemplateAndCitesLiveCode(t *testing.T) {
	doc := readReadme(t)
	root := acsassert.RepoRoot(t)

	sixOne, ok := section(doc, "### 6.1 ")
	if !ok {
		t.Fatalf("RED: §6.1 not found — cannot derive the issue/gap/solution template")
	}
	markerRe := regexp.MustCompile(`(?m)^\*\*([A-Z][^*]{0,40}?)\.\*\*`)
	var template []string
	for _, m := range markerRe.FindAllStringSubmatch(sixOne, -1) {
		switch strings.ToLower(m[1]) {
		case "issue", "gap", "solution":
			template = append(template, m[1])
		}
	}
	if len(template) < 3 {
		t.Fatalf("RED: could not derive issue/gap/solution markers from §6.1 (found %v)", template)
	}

	sixThree, ok := section(doc, "### 6.3")
	if !ok {
		t.Fatalf("RED: §6.3 subsection missing from %s — the cross-reference entry was not added", readmeRelPath)
	}
	for _, marker := range template {
		if !strings.Contains(sixThree, "**"+marker+".**") {
			t.Errorf("RED: §6.3 omits the **%s.** marker that §6.1 establishes as the §3.8 template", marker)
		}
	}

	norm := normalize(sixThree)
	if !strings.Contains(norm, salvageCodeRelPath) {
		t.Errorf("RED: §6.3 does not point at the producing code %s", salvageCodeRelPath)
	}
	if !acsassert.FileExists(t, filepath.Join(root, salvageCodeRelPath)) {
		t.Errorf("RED: §6.3's cited code path %s does not exist on disk — dangling cross-reference", salvageCodeRelPath)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", salvageCodeRelPath); code != 0 {
		t.Errorf("RED: %s is not git-tracked — §6.3 cites a path that will not survive ship", salvageCodeRelPath)
	}
	if !strings.Contains(norm, "§7") {
		t.Errorf("RED: §6.3 does not cross-reference §7 as the home of the measurement it summarises")
	}
}

// TestC1437_005_Section6InventsNoCountsBeyondSection7 is the anti-invention
// (OOD) axis. Every ratio (`A/B`) and percentage token appearing in §6's prose
// or §6.3 must be licensed by §7: percentages verbatim, ratios by both of their
// components. This is what stops the task being "satisfied" by a plausible but
// fabricated figure — the exact risk flagged in scout's hypotheses.
func TestC1437_005_Section6InventsNoCountsBeyondSection7(t *testing.T) {
	doc := readReadme(t)
	sec7 := normalize(section7(t, doc))
	scope := normalize(section6Intro(t, doc))
	if sixThree, ok := section(doc, "### 6.3"); ok {
		scope += " " + normalize(sixThree)
	}

	licensed := func(tok string) bool {
		return regexp.MustCompile(`(^|[^0-9.])` + regexp.QuoteMeta(tok) + `([^0-9.]|$)`).MatchString(sec7)
	}

	pctRe := regexp.MustCompile(`\d+(?:\.\d+)?%`)
	for _, pct := range pctRe.FindAllString(scope, -1) {
		if !strings.Contains(sec7, pct) {
			t.Errorf("RED: §6 states percentage %s which §7's measured table does not license — no figure may be invented here", pct)
		}
	}

	ratioRe := regexp.MustCompile(`(\d+)/(\d+)`)
	for _, m := range ratioRe.FindAllStringSubmatch(scope, -1) {
		if !licensed(m[1]) || !licensed(m[2]) {
			t.Errorf("RED: §6 states ratio %s whose components are not both present in §7's measured table — no figure may be invented here", m[0])
		}
	}
}
