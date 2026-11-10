//go:build acs

package cycle2265

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var planFiles = []string{
	"docs/plans/agent-messaging-2026-10.md",
	"docs/plans/convergence-policy-2026-10.md",
	"docs/plans/logging-and-process-hygiene-2026-10.md",
	"docs/plans/model-level-benches-2026-10.md",
	"docs/plans/retry-ladder-2026-10.md",
	"docs/plans/test-system-efficiency-2026-10.md",
}

var (
	hexToken    = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	prReference = regexp.MustCompile(`#\d{2,6}\b`)
	rowCellZero = regexp.MustCompile(`^(D|V-D|CQ|Q|K|R|F)\d+[a-z]?$`)
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root}, args...)...)
	if err != nil || code != 0 {
		t.Fatalf("git %s rc=%d err=%v stderr=%q", strings.Join(args, " "), code, err, acsassert.OutputTail(errOut, 5))
	}
	return strings.TrimSpace(out)
}

func readPlan(t *testing.T, rel string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("plan %s is unreadable: %v", rel, err)
	}
	return strings.Split(string(raw), "\n")
}

func splitRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	cells := strings.Split(trimmed, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func isTableRow(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

func statusColumn(header []string) int {
	for i, cell := range header {
		if strings.EqualFold(cell, "Status") {
			return i
		}
	}
	return -1
}

func statusCell(t *testing.T, rel, rowKey string) string {
	t.Helper()
	var header []string
	fallback := ""
	for _, line := range readPlan(t, rel) {
		if !isTableRow(line) {
			header = nil
			continue
		}
		cells := splitRow(line)
		if header == nil {
			header = cells
			continue
		}
		if strings.Trim(cells[0], "-: ") == "" || strings.Trim(cells[0], "`* ") != rowKey {
			continue
		}
		if col := statusColumn(header); col >= 0 && col < len(cells) {
			return cells[col]
		}
		if fallback == "" {
			fallback = line
		}
	}
	if fallback == "" {
		t.Fatalf("%s has no table row %q", rel, rowKey)
	}
	return fallback
}

func headerStatusLine(t *testing.T, rel string) string {
	t.Helper()
	for _, line := range readPlan(t, rel) {
		if strings.Contains(line, "Status:") && (strings.HasPrefix(line, ">") || strings.HasPrefix(line, "- ")) {
			return line
		}
	}
	t.Fatalf("%s has no header status line", rel)
	return ""
}

func citedCommits(cell string) []string {
	var shas []string
	for _, token := range hexToken.FindAllString(cell, -1) {
		hasDigit := strings.ContainsAny(token, "0123456789")
		hasLetter := strings.ContainsAny(token, "abcdef")
		if hasDigit && hasLetter {
			shas = append(shas, token)
		}
	}
	return shas
}

func requireCitation(t *testing.T, label, cell string) []string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	shas := citedCommits(cell)
	if len(shas) == 0 && !prReference.MatchString(cell) {
		t.Errorf("RED: %s cites no commit or PR: %q", label, cell)
		return nil
	}
	for _, sha := range shas {
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "--is-ancestor", sha+"^{commit}", "HEAD"); code != 0 {
			t.Errorf("RED: %s cites %s, which is not a commit on this branch", label, sha)
		}
	}
	return shas
}

func requireLanded(t *testing.T, label, cell string) {
	t.Helper()
	for _, open := range []string{"☐", "◐", "not started", "Staged", "H2 lane", "proposed", "not built"} {
		if strings.Contains(cell, open) {
			t.Errorf("RED: %s still reads %q: %q", label, open, cell)
		}
	}
	requireCitation(t, label, cell)
}

func baseRef(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	for _, ref := range []string{"origin/main", "main"} {
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "rev-parse", "--verify", "-q", ref); code == 0 {
			return git(t, root, "merge-base", "HEAD", ref)
		}
	}
	t.Fatalf("neither origin/main nor main resolves")
	return ""
}

func TestC2265_001_AgentMessagingM4RowIsBuiltAndCitesItsCommit(t *testing.T) {
	cell := statusCell(t, planFiles[0], "M4")
	if strings.Contains(cell, "not started") || strings.Contains(cell, "☐") {
		t.Errorf("RED: M4 row still reads not started: %q", cell)
	}
	if !strings.Contains(cell, "☑") {
		t.Errorf("RED: M4 row has no landed mark: %q", cell)
	}
	shas := requireCitation(t, "agent-messaging M4", cell)
	root := acsassert.RepoRoot(t)
	touchesCLI := false
	for _, sha := range shas {
		files, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "show", "--name-only", "--format=", sha)
		if code == 0 && strings.Contains(files, "go/cmd/evolve/cmd_msg.go") {
			touchesCLI = true
		}
	}
	if !touchesCLI {
		t.Errorf("RED: no commit cited by the M4 row changes go/cmd/evolve/cmd_msg.go: %v", shas)
	}
}

func TestC2265_002_AgentMessagingHeaderNoLongerCallsM4Open(t *testing.T) {
	header := headerStatusLine(t, planFiles[0])
	if regexp.MustCompile(`M4[^.;]*\bopen\b`).MatchString(header) {
		t.Errorf("RED: header status still lists M4 as open: %q", header)
	}
}

func TestC2265_003_ConvergenceV0V3bV4RowsAreLandedWithCitations(t *testing.T) {
	for _, row := range []string{"V0", "V3b", "V4"} {
		cell := statusCell(t, planFiles[1], row)
		if !strings.Contains(cell, "☑") {
			t.Errorf("RED: convergence %s row has no landed mark: %q", row, cell)
		}
		requireLanded(t, "convergence "+row, cell)
	}
}

func TestC2265_004_LoggingH1AndH2RowsAreMergedWithCitations(t *testing.T) {
	for _, row := range []string{"H1", "H2"} {
		cell := statusCell(t, planFiles[2], row)
		if !regexp.MustCompile(`(?i)\bmerged\b`).MatchString(cell) {
			t.Errorf("RED: logging %s row does not say merged: %q", row, cell)
		}
		requireLanded(t, "logging "+row, cell)
	}
}

func TestC2265_005_ModelLevelBenchesHeaderSaysStepsOneAndTwoLanded(t *testing.T) {
	header := headerStatusLine(t, planFiles[3])
	if strings.Contains(header, "not built") {
		t.Errorf("RED: header still says not built: %q", header)
	}
	stepsOneTwo := regexp.MustCompile(`(?i)steps?\s*1\s*(?:–|-|to|and|,)\s*2`)
	landedWord := regexp.MustCompile(`(?i)\b(landed|built|merged)\b`)
	if !stepsOneTwo.MatchString(header) || !landedWord.MatchString(header) {
		t.Errorf("RED: header does not say that steps 1 and 2 landed: %q", header)
	}
	requireCitation(t, "model-level-benches header", header)
}

func TestC2265_006_RetryLadderAuditFixRowCitesPullRequest1236(t *testing.T) {
	cell := statusCell(t, planFiles[4], "audit_fix")
	if !strings.Contains(cell, "#1236") {
		t.Errorf("RED: audit_fix status cell does not cite #1236: %q", cell)
	}
	requireCitation(t, "retry-ladder audit_fix", cell)
}

func TestC2265_007_TestSystemEfficiencyT6RowIsBuiltAndCitesItsCommit(t *testing.T) {
	cell := statusCell(t, planFiles[5], "T6")
	if strings.Contains(cell, "Status: open") {
		t.Errorf("RED: T6 row still reads Status: open: %q", cell)
	}
	if !regexp.MustCompile(`Status: (built|done|consumed)`).MatchString(cell) {
		t.Errorf("RED: T6 row has no built status: %q", cell)
	}
	if !strings.Contains(cell, "66b0bcc7e") {
		t.Errorf("RED: T6 row does not cite 66b0bcc7e: %q", cell)
	}
	requireCitation(t, "test-system-efficiency T6", cell)
}

func TestC2265_008_AllSixPlansChangeAndNoDecisionRowIsRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base := baseRef(t)
	names := git(t, root, append([]string{"diff", "--name-only", base, "--"}, planFiles...)...)
	for _, rel := range planFiles {
		if !strings.Contains(names, rel) {
			t.Errorf("RED: %s has no change against the base", rel)
		}
	}
	diff := git(t, root, append([]string{"diff", "-U0", base, "--"}, planFiles...)...)
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "-|") {
			continue
		}
		cells := splitRow(strings.TrimPrefix(line, "-"))
		key := strings.Trim(cells[0], "`* ")
		if rowCellZero.MatchString(key) {
			t.Errorf("RED: a decision or fact row changed: %q", line)
		}
	}
}

func TestC2265_009_SteLintPassesOnTheAddedLinesOfTheChangedPlans(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base := baseRef(t)
	names := git(t, root, append([]string{"diff", "--name-only", base, "--"}, planFiles...)...)
	if strings.TrimSpace(names) == "" {
		t.Fatalf("RED: no plan file changed against %s, so there is nothing to lint", base)
	}
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"docs", "ste-lint", "--strict", "--project-root", root, "--changed", base, "--added")
	if err != nil {
		t.Fatalf("ste-lint did not run: %v", err)
	}
	if code != 0 || !strings.Contains(out, "ste-lint: 0 finding(s)") {
		t.Errorf("ste-lint rc=%d out=%q stderr=%q", code, acsassert.OutputTail(out, 20), acsassert.OutputTail(errOut, 5))
	}
	if !strings.Contains(out, "file(s) checked") || strings.Contains(out, "; 0 file(s) checked") {
		t.Errorf("RED: ste-lint checked no file: %q", acsassert.OutputTail(out, 5))
	}
}
