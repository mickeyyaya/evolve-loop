//go:build acs

package cycle1437

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const readmeRelPath = "docs/research/deliverable-alignment-2026-08/README.md"

const salvageCodeRelPath = "go/internal/deliverable/salvage_instrument.go"

const stalePlaceholder = "not yet instrumented"

func readReadme(t *testing.T) string {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), readmeRelPath)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("RED: cannot read target document %s: %v", readmeRelPath, err)
	}
	return string(b)
}

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

func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

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
	if p := pathRe.FindString(intro); p != "" && p != evidence {
		t.Errorf("RED: §6 cites evidence %s but §7's source of record is %s — the citations disagree", p, evidence)
	}
}

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
