//go:build acs

// harness_test.go — the cycle-1648 predicate harness: the real `evolve` binary
// built once, the fixture corpus (five valid pairs, five named exclusions, two
// out-of-sample entries), and the Markdown report-shape matchers. Split from
// predicates_test.go so the contract file stays readable; both carry the acs
// build tag so neither compiles into the normal suite.
package cycle1648

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ---------------------------------------------------------------------------
// Harness: the real CLI, built once.
// ---------------------------------------------------------------------------

var (
	evolveBin      string
	evolveBuildErr error
)

// TestMain compiles go/cmd/evolve ONCE for the whole predicate package (the
// cycle-1439 buildEvolve shape, hoisted so nine CLI predicates don't relink nine
// times under fleet load). `go -C` pins the module root to this worktree so the
// binary is built from the tree the ship lands, never from process cwd.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1648-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "acs/cycle1648: tempdir:", err)
		os.Exit(1)
	}
	evolveBin = filepath.Join(dir, "evolve")
	root, err := repoRootFromCwd()
	if err != nil {
		evolveBuildErr = err
	} else {
		out, err := exec.Command("go", "-C", filepath.Join(root, "go"), "build", "-o", evolveBin, "./cmd/evolve").CombinedOutput()
		if err != nil {
			evolveBuildErr = fmt.Errorf("go build ./cmd/evolve: %v\n%s", err, out)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// repoRootFromCwd mirrors acsassert.RepoRoot for TestMain (no *testing.T yet):
// `git -C <cwd> rev-parse --show-toplevel`, so the worktree resolves from the
// package directory `go test` runs in, not from the fleet lane's shell cwd.
func repoRootFromCwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	if evolveBuildErr != nil {
		t.Fatalf("RED (harness): %v", evolveBuildErr)
	}
	return evolveBin
}

// stateRoot resolves the MAIN project root (runtime state under .evolve/) —
// the dual-root idiom (go/acs/README.md): EVOLVE_PROJECT_ROOT → state.
func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

// runEvolve runs the real binary from a NEUTRAL cwd (a throwaway temp dir) so a
// command that silently resolves the corpus from process cwd cannot pass.
func runEvolve(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(evolveBinary(t), args...)
	cmd.Dir = t.TempDir()
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("exec %v: %v", args, err)
	}
	return so.String(), se.String(), code
}

// calibrate runs `evolve audit calibration` with explicit corpus/output paths
// and returns the report text (empty when the file was not written).
func calibrate(t *testing.T, dossiers, runs string, extra ...string) (report, stderr string, code int) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "calibration.md")
	args := append([]string{"audit", "calibration", "--dossiers-dir", dossiers, "--runs-dir", runs, "--output", out}, extra...)
	_, stderr, code = runEvolve(t, args...)
	if b, err := os.ReadFile(out); err == nil {
		report = string(b)
	}
	return report, stderr, code
}

// ---------------------------------------------------------------------------
// Fixture corpus.
// ---------------------------------------------------------------------------

const (
	gateExplain = "explanation documentation qualitative review"
	gateEGPS    = "EGPS ship_eligible=false"
	gateAPI     = "apicover-enforce gate"

	reasonExplain  = "explanation review Evidence must cite go/internal/core/phase.go with path:line evidence"
	reasonEGPS     = "EGPS: acs-verdict.json ship_eligible=false — the authoritative acssuite SSOT rejects the ship even though red_count=0"
	reasonConflict = "verdict-conflict: auditor narrative=%s but %d deterministic gate(s) forced FAIL [%s] — the gate outranks the narrative (ship policy unchanged); both readings are recorded so the disagreement is weighable."
	reasonRetro    = "phase audit verdict FAIL routed to retro (agent-graded; see the audit report artifact) defect=H1 (HIGH): the builder never ran the suite"
)

// shadowJSON renders an audit-chain-shadow.json in the exact shape
// internal/auditchain.ShadowRecord marshals (cycle-1640's record is the model).
// shipped=="" omits the field (omitempty), the legacy-record shape.
func shadowJSON(cycle int, narrative, chain, shipped string, overrodeBy []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "{\n  \"cycle\": %d,\n  \"phase\": \"audit\",\n  \"chain_present\": %v,\n  \"narrative_verdict\": %q,\n", cycle, chain != "absent", narrative)
	if shipped != "" {
		fmt.Fprintf(&b, "  \"shipped_verdict\": %q,\n", shipped)
	}
	if len(overrodeBy) > 0 {
		quoted := make([]string, len(overrodeBy))
		for i, g := range overrodeBy {
			quoted[i] = strconv.Quote(g)
		}
		fmt.Fprintf(&b, "  \"overrode_by\": [%s],\n", strings.Join(quoted, ", "))
	}
	fmt.Fprintf(&b, "  \"chain_verdict\": %q,\n  \"agrees\": %v,\n  \"rationale\": \"fixture\"\n}\n", chain, strings.EqualFold(narrative, chain))
	return b.String()
}

// dossierJSON renders a knowledge-base dossier that satisfies
// internal/dossier.Validate (goal, >=1 phase; a FAIL carries a defect and a
// carryover) so a reader that validates and one that only decodes agree.
func dossierJSON(cycle int, finalVerdict string) string {
	extra := ""
	if finalVerdict == "FAIL" {
		extra = `,
  "defects": [{"id": "audit-fail", "severity": "HIGH", "summary": "cycle did not pass audit", "fix": "address the audit findings"}],
  "carryover": [{"id": "address-audit-findings", "action": "address the audit findings recorded for this cycle"}]`
	}
	return fmt.Sprintf(`{
  "cycle": %d,
  "run_id": "01FIXTURE%d",
  "goal": "fixture goal",
  "final_verdict": %q,
  "phases": [{"name": "audit", "verdict": %q}]%s
}
`, cycle, cycle, finalVerdict, finalVerdict, extra)
}

func reasonsJSON(reasons ...string) string {
	quoted := make([]string, len(reasons))
	for i, r := range reasons {
		quoted[i] = strconv.Quote(r)
	}
	return fmt.Sprintf("{\n  \"schema_version\": 1,\n  \"phase\": \"audit\",\n  \"reasons\": [%s]\n}\n", strings.Join(quoted, ", "))
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// corpus is one fixture corpus: dossiers under Dossiers, run dirs under Runs.
type corpus struct{ Dossiers, Runs string }

func newCorpus(t *testing.T) corpus {
	t.Helper()
	base := t.TempDir()
	c := corpus{Dossiers: filepath.Join(base, "knowledge-base", "cycles"), Runs: filepath.Join(base, ".evolve", "runs")}
	for _, d := range []string{c.Dossiers, c.Runs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func (c corpus) dossier(t *testing.T, cycle int, body string) {
	mustWrite(t, filepath.Join(c.Dossiers, fmt.Sprintf("cycle-%d.json", cycle)), body)
}
func (c corpus) shadow(t *testing.T, name, body string) {
	mustWrite(t, filepath.Join(c.Runs, name, "audit-chain-shadow.json"), body)
}
func (c corpus) reasons(t *testing.T, name, body string) {
	mustWrite(t, filepath.Join(c.Runs, name, "audit-fail-reason.json"), body)
}

// fixtureCorpus is the canonical corpus every representation predicate reads.
//
//	VALID PAIRS (5)                narrative chain   gate  shipped  overrode_by
//	  101 clean pass                PASS      PASS    PASS  PASS     —
//	  102 force-override (AC4 edge) PASS      PASS    FAIL  FAIL     explanation review
//	  103 WARN under a red gate     WARN      absent  FAIL  FAIL     EGPS
//	  104 two gates                 PASS      PASS    FAIL  FAIL     EGPS + apicover
//	  105 narrative FAIL, gates     FAIL      FAIL    PASS  FAIL     —   (shipped_verdict
//	      green                                                          absent → dossier)
//	EXCLUSIONS (5)
//	  106 dossier, run dir with no shadow            → missing-shadow
//	  107 dossier, shadow is not JSON                → malformed-shadow
//	  108 shadow, no dossier                         → missing-dossier
//	  109 shadow, dossier is truncated JSON          → malformed-dossier
//	  110 dossier + shadow, fail-reason is not JSON  → malformed-fail-reason
//	OUT OF SAMPLE (neither pair nor exclusion)
//	  cycle-1623.reset-20260911T180942 (non-canonical run name, valid shadow)
//	  cycle-111.json (a dossier with no run dir — no narrative can exist)
//
// Expected matrix (narrative × gate): (PASS,PASS)=1 (PASS,FAIL)=2 (WARN,FAIL)=1
// (FAIL,PASS)=1. Expected classes: EGPS=2 (103,104), verdict-conflict=2 (102,103).
func fixtureCorpus(t *testing.T) corpus {
	t.Helper()
	c := newCorpus(t)
	// 101
	c.dossier(t, 101, dossierJSON(101, "PASS"))
	c.shadow(t, "cycle-101", shadowJSON(101, "PASS", "PASS", "PASS", nil))
	// 102
	c.dossier(t, 102, dossierJSON(102, "FAIL"))
	c.shadow(t, "cycle-102", shadowJSON(102, "PASS", "PASS", "FAIL", []string{gateExplain}))
	c.reasons(t, "cycle-102", reasonsJSON(reasonExplain, fmt.Sprintf(reasonConflict, "PASS", 1, gateExplain)))
	// 103
	c.dossier(t, 103, dossierJSON(103, "FAIL"))
	c.shadow(t, "cycle-103", shadowJSON(103, "WARN", "absent", "FAIL", []string{gateEGPS}))
	c.reasons(t, "cycle-103", reasonsJSON(reasonEGPS, fmt.Sprintf(reasonConflict, "WARN", 1, gateEGPS)))
	// 104
	c.dossier(t, 104, dossierJSON(104, "FAIL"))
	c.shadow(t, "cycle-104", shadowJSON(104, "PASS", "PASS", "FAIL", []string{gateEGPS, gateAPI}))
	c.reasons(t, "cycle-104", reasonsJSON(reasonEGPS))
	// 105 — legacy shadow without shipped_verdict; the dossier supplies it.
	c.dossier(t, 105, dossierJSON(105, "FAIL"))
	c.shadow(t, "cycle-105", shadowJSON(105, "FAIL", "FAIL", "", nil))
	c.reasons(t, "cycle-105", reasonsJSON(reasonRetro))
	// 106 — run dir exists (a triage-failed cycle: no audit ran), no shadow.
	c.dossier(t, 106, dossierJSON(106, "FAIL"))
	mustWrite(t, filepath.Join(c.Runs, "cycle-106", "triage-report.md"), "# no audit ran\n")
	// 107
	c.dossier(t, 107, dossierJSON(107, "PASS"))
	c.shadow(t, "cycle-107", "{ this is not json\n")
	// 108
	c.shadow(t, "cycle-108", shadowJSON(108, "PASS", "PASS", "PASS", nil))
	// 109
	c.dossier(t, 109, `{"cycle": 109, "final_verdict": "PASS", "phases": [`)
	c.shadow(t, "cycle-109", shadowJSON(109, "PASS", "PASS", "PASS", nil))
	// 110
	c.dossier(t, 110, dossierJSON(110, "FAIL"))
	c.shadow(t, "cycle-110", shadowJSON(110, "FAIL", "FAIL", "FAIL", nil))
	c.reasons(t, "cycle-110", `{"schema_version": 1, "reasons": "not-an-array"`)
	// out of sample
	c.shadow(t, "cycle-1623.reset-20260911T180942.964616000", shadowJSON(1623, "WARN", "FAIL", "WARN", nil))
	c.dossier(t, 111, dossierJSON(111, "PASS"))
	return c
}

// ---------------------------------------------------------------------------
// Report-shape helpers.
// ---------------------------------------------------------------------------

// row builds a start-anchored Markdown table-row matcher `| c1 | c2 | …`
// tolerant of cell padding; cells AFTER the given ones are free (a prefix
// match). An empty string matches an empty cell `|  |`.
func row(cells ...string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString(`(?m)^\|`)
	for _, c := range cells {
		b.WriteString(`\s*` + regexp.QuoteMeta(c) + `\s*\|`)
	}
	return regexp.MustCompile(b.String())
}

// anyCell is a wildcard for one free cell in rowExact.
const anyCell = "\x00any"

// rowExact is row() anchored at BOTH ends: the row has exactly these cells.
// Needed wherever two tables share leading cells (an override row and a pairs
// row both start `| 102 | PASS |`); anyCell frees one cell.
func rowExact(cells ...string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString(`(?m)^\|`)
	for _, c := range cells {
		if c == anyCell {
			b.WriteString(`[^|]*\|`)
			continue
		}
		b.WriteString(`\s*` + regexp.QuoteMeta(c) + `\s*\|`)
	}
	b.WriteString(`\s*$`)
	return regexp.MustCompile(b.String())
}

func requireRow(t *testing.T, report, what string, re *regexp.Regexp) {
	t.Helper()
	if !re.MatchString(report) {
		t.Errorf("RED: %s — no row matching %s in report:\n%s", what, re, report)
	}
}

func forbidRow(t *testing.T, report, what string, re *regexp.Regexp) {
	t.Helper()
	if m := re.FindString(report); m != "" {
		t.Errorf("RED: %s — unexpected row %q", what, m)
	}
}

// findRow returns the WHOLE first line on which re matches ("" when none) —
// row() is a prefix matcher, so FindString alone would truncate the line at the
// last pinned cell and hide the free cells after it.
func findRow(report string, re *regexp.Regexp) string {
	loc := re.FindStringIndex(report)
	if loc == nil {
		return ""
	}
	rest := report[loc[0]:]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		return rest[:nl]
	}
	return rest
}

// Row shapes of the two per-cycle tables, used to read cycle numbers in
// document order (ordering must be checked WITHIN a table: the override table
// also lists cycles, so a global first-index comparison is ambiguous).
var (
	// | Cycle | Narrative | Chain | Gate | Shipped | Overrode By | Defect Classes |
	pairRowRe = regexp.MustCompile(`(?m)^\|\s*(\d+)\s*\|\s*(?:PASS|WARN|FAIL)\s*\|[^|]*\|\s*(?:PASS|FAIL)\s*\|\s*(?:PASS|WARN|FAIL)\s*\|[^|]*\|[^|]*\|\s*$`)
	// | Cycle | Reason | Detail |
	exclusionRowRe = regexp.MustCompile(`(?m)^\|\s*(\d+)\s*\|\s*(?:missing|malformed)-[a-z-]+\s*\|[^|]*\|\s*$`)
)

// cyclesInOrder returns the cycle numbers of every row matching re, in
// document order.
func cyclesInOrder(report string, re *regexp.Regexp) []int {
	var out []int
	for _, m := range re.FindAllStringSubmatch(report, -1) {
		n, _ := strconv.Atoi(m[1])
		out = append(out, n)
	}
	return out
}

func isAscending(xs []int) bool {
	for i := 1; i < len(xs); i++ {
		if xs[i] <= xs[i-1] {
			return false
		}
	}
	return true
}

// summaryCount reads `<label>: N` from the report (-1 when absent).
func summaryCount(report, label string) int {
	m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(label) + `:\s*(\d+)\s*$`).FindStringSubmatch(report)
	if m == nil {
		return -1
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// firstLine trims a stderr dump to its first non-empty line for messages (the
// dispatcher's usage block is ~130 lines).
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

// matrixRows are the `| <narrative> | <gate> | <count> |` rows of the agreement
// matrix — the ONLY three-column rows whose first two cells are verdicts.
var matrixRowRe = regexp.MustCompile(`(?m)^\|\s*(PASS|WARN|FAIL)\s*\|\s*(PASS|FAIL)\s*\|\s*(\d+)\s*\|\s*$`)

func matrixTotal(report string) int {
	total := 0
	for _, m := range matrixRowRe.FindAllStringSubmatch(report, -1) {
		n, _ := strconv.Atoi(m[3])
		total += n
	}
	return total
}
