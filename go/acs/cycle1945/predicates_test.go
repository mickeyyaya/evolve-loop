//go:build acs

package cycle1945

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	designDoc  = "docs/architecture/audit-evaluation-contract.md"
	planDoc    = "docs/plans/audit-evaluation-contract-2026-10.md"
	dossierDoc = "docs/research/audit-evaluation-contract-prior-art-2026-10.md"
	indexDoc   = "docs/architecture/review-loop-and-quality-index.md"
	followupID = "audit-contract-e0-review-followups"
)

var touchedDocs = []string{designDoc, planDoc, dossierDoc, indexDoc}

var markdownLink = regexp.MustCompile(`\]\(([^)\s]+)\)`)

func sourceRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv(ipcenv.WorktreeRootKey); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

func readDoc(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(sourceRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func section(t *testing.T, doc, startMarker, endMarker string) string {
	t.Helper()
	i := strings.Index(doc, startMarker)
	if i < 0 {
		t.Fatalf("section start %q not found", startMarker)
	}
	rest := doc[i+len(startMarker):]
	if j := strings.Index(rest, endMarker); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

func lineWith(doc string, needles ...string) string {
	for _, line := range strings.Split(doc, "\n") {
		ok := true
		for _, n := range needles {
			if !strings.Contains(line, n) {
				ok = false
				break
			}
		}
		if ok {
			return line
		}
	}
	return ""
}

func brokenRelativeLinks(root, rel, body string) []string {
	var broken []string
	dir := filepath.Dir(filepath.Join(root, rel))
	for _, m := range markdownLink.FindAllStringSubmatch(body, -1) {
		target := m[1]
		if strings.Contains(target, "://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		if i := strings.IndexAny(target, "#?"); i >= 0 {
			target = target[:i]
		}
		if target == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, target)); err != nil {
			broken = append(broken, m[1])
		}
	}
	return broken
}

func TestC1945_001_V1_InstrumentFenceResnapshotsAfterEveryTddDispatch(t *testing.T) {
	// acs-predicate: config-check
	body := section(t, readDoc(t, designDoc), "#### 4.2", "\n#### 4.3")
	line := lineWith(body, "re-snapshot")
	if line == "" {
		t.Fatalf("section 4.2 has no line saying the fence re-snapshots")
	}
	for _, want := range []string{"tdd", "audit-repair", "testFiles", "go/acs/cycle<N>/**", "eval file"} {
		if !strings.Contains(line, want) {
			t.Errorf("re-snapshot line lacks %q: %s", want, line)
		}
	}
}

func TestC1945_002_V2_DegradeRuleIsPreShipFloorAndNamesSkipSignalField(t *testing.T) {
	// acs-predicate: config-check
	body := section(t, readDoc(t, designDoc), "**Exhaustion degrades; it never aborts.**", "\n### ")
	for _, want := range []string{"before the ship floor", "skip-signal field"} {
		if !strings.Contains(body, want) {
			t.Errorf("degrade rule lacks %q", want)
		}
	}
}

func TestC1945_003_V3_FindingFieldsAddKindBlockingAndCertificateKindPlaceholder(t *testing.T) {
	// acs-predicate: config-check
	doc := readDoc(t, designDoc)
	fields := lineWith(doc, "**Findings** use the shared grammar")
	if fields == "" {
		t.Fatalf("findings grammar line missing")
	}
	for _, want := range []string{"Kind", "Blocking", "ADR-0126", "Certificate: <certificate-kind>"} {
		if !strings.Contains(fields, want) {
			t.Errorf("findings fields line lacks %q: %s", want, fields)
		}
	}
	if strings.Contains(doc, "Certificate: <kind>") {
		t.Errorf("the placeholder <kind> still names a certificate; rename it <certificate-kind>")
	}
}

func TestC1945_004_V5_CertificateFormMayCombineKindsAndGCommentResolvedByStage(t *testing.T) {
	// acs-predicate: config-check
	body := section(t, readDoc(t, designDoc), "#### 2.2 Certificate kinds", "\n### ")
	if !strings.Contains(body, "may combine two kinds") {
		t.Errorf("section 2.2 does not say a certificate form may combine two kinds")
	}
	line := lineWith(body, "G-COMMENT", "resolved by stage")
	if line == "" {
		t.Errorf("section 2.2 does not say G-COMMENT's form is resolved by stage")
	} else if !strings.Contains(line, "check 6") {
		t.Errorf("G-COMMENT line does not tie stage resolution to check 6: %s", line)
	}
}

func TestC1945_005_V6_ExpectationsDimensionsLineCarriesDepth(t *testing.T) {
	// acs-predicate: config-check
	doc := readDoc(t, designDoc)
	line := lineWith(doc, "**`## Dimensions`** has one line per quality-index key")
	if line == "" {
		t.Fatalf("Dimensions format line missing")
	}
	carriesDepth := strings.Contains(line, "depth")
	personaRule := regexp.MustCompile(`(?i)depth[^\n]*persona rule|persona rule[^\n]*depth`).MatchString(doc)
	if !carriesDepth && !personaRule {
		t.Errorf("neither the Dimensions line carries a depth nor a persona rule defines it")
	}
}

func TestC1945_006_V7_EDecision10RecordsGReviewRemovalAndGAcceptEval(t *testing.T) {
	// acs-predicate: config-check
	row := lineWith(readDoc(t, planDoc), "| E-D10 |")
	if row == "" {
		t.Fatalf("no E-D10 row in the plan's decision table")
	}
	for _, want := range []string{"G-REVIEW", "G-ACCEPT.<task>.eval", "removed", "added"} {
		if !strings.Contains(row, want) {
			t.Errorf("E-D10 row lacks %q: %s", want, row)
		}
	}
}

func TestC1945_007_V8_DossierVerifierLineNamesVerifierSetAndED9(t *testing.T) {
	// acs-predicate: config-check
	line := lineWith(readDoc(t, dossierDoc), "**Shaped:** AR3.")
	if line == "" {
		t.Fatalf("dossier AR3 line missing")
	}
	for _, want := range []string{"Verifier: both", "E-D9"} {
		if !strings.Contains(line, want) {
			t.Errorf("AR3 line lacks %q: %s", want, line)
		}
	}
}

func TestC1945_008_V9_DesignSection10StatesADR0126Interplay(t *testing.T) {
	// acs-predicate: config-check
	body := section(t, readDoc(t, designDoc), "### 10. Interplay with existing mechanisms", "\n### 11.")
	for _, want := range []string{"ADR-0126", "rung 2(a)", "distilled brief", "verify-only"} {
		if !strings.Contains(body, want) {
			t.Errorf("section 10 lacks %q", want)
		}
	}
}

func TestC1945_009_V11_ToleranceLineCitesTheParsers(t *testing.T) {
	// acs-predicate: config-check
	line := lineWith(readDoc(t, designDoc), "**Tolerances**")
	if line == "" {
		t.Fatalf("Tolerances line missing")
	}
	for _, want := range []string{"qualityindex", "reportdoc.Fields"} {
		if !strings.Contains(line, want) {
			t.Errorf("Tolerances line does not cite %q: %s", want, line)
		}
	}
}

func TestC1945_010_V13_FourCriteriaIndexRLQualificationConditions(t *testing.T) {
	// acs-predicate: config-check
	doc := readDoc(t, designDoc)
	line := lineWith(doc, "one flip and one signal per defect")
	if line == "" {
		t.Fatalf("no line states one flip and one signal per defect")
	}
	for _, want := range []string{"G-ACS", "G-EVAL", "G-ACCEPT", "G-DISPO", "RL §6.2"} {
		if !strings.Contains(line, want) {
			t.Errorf("qualification line lacks %q: %s", want, line)
		}
	}
}

func TestC1945_011_V14_CriterionFindingCountsForItsDimensionAndCapIsAuditScoresOnly(t *testing.T) {
	// acs-predicate: config-check
	doc := readDoc(t, designDoc)
	count := lineWith(doc, "Criterion finding counts for its")
	if count == "" || !strings.Contains(count, "§2.1") {
		t.Errorf("no line saying a Criterion finding counts for its §2.1 dimension: %q", count)
	}
	if lineWith(doc, "cap applies to the audit's Scores only") == "" {
		t.Errorf("no line limiting the cap to the audit's Scores")
	}
}

func TestC1945_012_IndexLinksADR0126AtTheConvergencePolicyCall(t *testing.T) {
	root := sourceRoot(t)
	index := readDoc(t, indexDoc)
	re := regexp.MustCompile(`\[[^\]]*\]\(([^)\s]*0126-every-iterative-loop-converges-or-escalates\.md)\)`)
	var linked bool
	for _, line := range strings.Split(index, "\n") {
		m := re.FindStringSubmatch(line)
		if m == nil || !strings.Contains(strings.ToLower(line), "converg") {
			continue
		}
		linked = true
		if _, err := os.Stat(filepath.Join(root, filepath.Dir(indexDoc), m[1])); err != nil {
			t.Errorf("ADR-0126 link %q does not resolve: %v", m[1], err)
		}
	}
	if !linked {
		t.Errorf("the index has no link to ADR-0126 on a line about the convergence policy")
	}
}

func TestC1945_013_LandingNotesCiteResolvingFileLineForEachResolvedFinding(t *testing.T) {
	root := sourceRoot(t)
	notes := section(t, readDoc(t, planDoc), "## 10. Landing notes", "\n## ")
	var row string
	for _, line := range strings.Split(notes, "\n") {
		if strings.Contains(line, followupID) && strings.Contains(line, "V14") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatalf("no landing-notes row for %s naming V14", followupID)
	}
	for _, v := range []string{"V1", "V2", "V3", "V5", "V6", "V7", "V8", "V9", "V11", "V13", "V14"} {
		if !regexp.MustCompile(`\b` + v + `\b`).MatchString(row) {
			t.Errorf("landing row does not mention %s", v)
		}
	}
	refs := regexp.MustCompile("(docs/[A-Za-z0-9_./-]+\\.md):(\\d+)").FindAllStringSubmatch(row, -1)
	if len(refs) < 11 {
		t.Errorf("landing row cites %d file:line refs, want at least 11 (one per resolved finding)", len(refs))
	}
	for _, r := range refs {
		body, err := os.ReadFile(filepath.Join(root, r[1]))
		if err != nil {
			t.Errorf("cited file %s: %v", r[1], err)
			continue
		}
		n, _ := strconv.Atoi(r[2])
		lines := strings.Count(string(body), "\n") + 1
		if n < 1 || n > lines {
			t.Errorf("%s:%d is outside the file (%d lines)", r[1], n, lines)
		}
	}
}

func TestC1945_014_EveryRelativeLinkInTheTouchedDocsResolves(t *testing.T) {
	root := sourceRoot(t)
	for _, rel := range touchedDocs {
		for _, b := range brokenRelativeLinks(root, rel, readDoc(t, rel)) {
			t.Errorf("%s: relative link %q does not resolve", rel, b)
		}
	}
}

func TestC1945_015_LinkCheckerRejectsAMissingTargetAndIgnoresExternalAndAnchorLinks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "real.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "[ok](real.md#sec) [gone](missing.md) [web](https://example.com/a.md) [self](#top)"
	got := brokenRelativeLinks(root, "doc.md", body)
	if len(got) != 1 || got[0] != "missing.md" {
		t.Errorf("broken links = %v, want exactly [missing.md]", got)
	}
}
