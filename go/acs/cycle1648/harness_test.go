//go:build acs

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

var (
	evolveBin      string
	evolveBuildErr error
)

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

func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

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

const (
	gateExplain = "explanation documentation qualitative review"
	gateEGPS    = "EGPS ship_eligible=false"
	gateAPI     = "apicover-enforce gate"

	reasonExplain  = "explanation review Evidence must cite go/internal/core/phase.go with path:line evidence"
	reasonEGPS     = "EGPS: acs-verdict.json ship_eligible=false — the authoritative acssuite SSOT rejects the ship even though red_count=0"
	reasonConflict = "verdict-conflict: auditor narrative=%s but %d deterministic gate(s) forced FAIL [%s] — the gate outranks the narrative (ship policy unchanged); both readings are recorded so the disagreement is weighable."
	reasonRetro    = "phase audit verdict FAIL routed to retro (agent-graded; see the audit report artifact) defect=H1 (HIGH): the builder never ran the suite"
)

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

func fixtureCorpus(t *testing.T) corpus {
	t.Helper()
	c := newCorpus(t)
	c.dossier(t, 101, dossierJSON(101, "PASS"))
	c.shadow(t, "cycle-101", shadowJSON(101, "PASS", "PASS", "PASS", nil))
	c.dossier(t, 102, dossierJSON(102, "FAIL"))
	c.shadow(t, "cycle-102", shadowJSON(102, "PASS", "PASS", "FAIL", []string{gateExplain}))
	c.reasons(t, "cycle-102", reasonsJSON(reasonExplain, fmt.Sprintf(reasonConflict, "PASS", 1, gateExplain)))
	c.dossier(t, 103, dossierJSON(103, "FAIL"))
	c.shadow(t, "cycle-103", shadowJSON(103, "WARN", "absent", "FAIL", []string{gateEGPS}))
	c.reasons(t, "cycle-103", reasonsJSON(reasonEGPS, fmt.Sprintf(reasonConflict, "WARN", 1, gateEGPS)))
	c.dossier(t, 104, dossierJSON(104, "FAIL"))
	c.shadow(t, "cycle-104", shadowJSON(104, "PASS", "PASS", "FAIL", []string{gateEGPS, gateAPI}))
	c.reasons(t, "cycle-104", reasonsJSON(reasonEGPS))
	c.dossier(t, 105, dossierJSON(105, "FAIL"))
	c.shadow(t, "cycle-105", shadowJSON(105, "FAIL", "FAIL", "", nil))
	c.reasons(t, "cycle-105", reasonsJSON(reasonRetro))
	c.dossier(t, 106, dossierJSON(106, "FAIL"))
	mustWrite(t, filepath.Join(c.Runs, "cycle-106", "triage-report.md"), "# no audit ran\n")
	c.dossier(t, 107, dossierJSON(107, "PASS"))
	c.shadow(t, "cycle-107", "{ this is not json\n")
	c.shadow(t, "cycle-108", shadowJSON(108, "PASS", "PASS", "PASS", nil))
	c.dossier(t, 109, `{"cycle": 109, "final_verdict": "PASS", "phases": [`)
	c.shadow(t, "cycle-109", shadowJSON(109, "PASS", "PASS", "PASS", nil))
	c.dossier(t, 110, dossierJSON(110, "FAIL"))
	c.shadow(t, "cycle-110", shadowJSON(110, "FAIL", "FAIL", "FAIL", nil))
	c.reasons(t, "cycle-110", `{"schema_version": 1, "reasons": "not-an-array"`)
	c.shadow(t, "cycle-1623.reset-20260911T180942.964616000", shadowJSON(1623, "WARN", "FAIL", "WARN", nil))
	c.dossier(t, 111, dossierJSON(111, "PASS"))
	return c
}

func row(cells ...string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString(`(?m)^\|`)
	for _, c := range cells {
		b.WriteString(`\s*` + regexp.QuoteMeta(c) + `\s*\|`)
	}
	return regexp.MustCompile(b.String())
}

const anyCell = "\x00any"

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

var (
	pairRowRe      = regexp.MustCompile(`(?m)^\|\s*(\d+)\s*\|\s*(?:PASS|WARN|FAIL)\s*\|[^|]*\|\s*(?:PASS|FAIL)\s*\|\s*(?:PASS|WARN|FAIL)\s*\|[^|]*\|[^|]*\|\s*$`)
	exclusionRowRe = regexp.MustCompile(`(?m)^\|\s*(\d+)\s*\|\s*(?:missing|malformed)-[a-z-]+\s*\|[^|]*\|\s*$`)
)

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

func summaryCount(report, label string) int {
	m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(label) + `:\s*(\d+)\s*$`).FindStringSubmatch(report)
	if m == nil {
		return -1
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

var matrixRowRe = regexp.MustCompile(`(?m)^\|\s*(PASS|WARN|FAIL)\s*\|\s*(PASS|FAIL)\s*\|\s*(\d+)\s*\|\s*$`)

func matrixTotal(report string) int {
	total := 0
	for _, m := range matrixRowRe.FindAllStringSubmatch(report, -1) {
		n, _ := strconv.Atoi(m[3])
		total += n
	}
	return total
}
