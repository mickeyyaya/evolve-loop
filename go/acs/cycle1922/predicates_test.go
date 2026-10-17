//go:build acs

package cycle1922

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	decompositionRel = "docs/architecture/decomposition/06-inboxmover.md"
	packageDocRel    = "docs/architecture/packages/internal-inboxmover.md"
	symbol           = "RecordRootTaskFailure"
)

var retirementMarkers = []string{"retired", "retirement", "deleted", "removed"}

var presentTenseClaims = []string{
	"stays as a second quarantine writer",
	"has no production caller today",
	"is the root-resident twin",
	"No test pins the lock",
	"takes the inbox-level `.bump.lock`",
	"an operator who later wires it",
}

func docLines(t *testing.T, rel string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(raw), "\n")
}

func isNumberedTestListItem(line string) bool {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	return i > 0 && strings.HasPrefix(line[i:], ". `Test")
}

func containsAny(line string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(line, n) {
			return true
		}
	}
	return false
}

func linesViolatingRetirementRecord(lines []string) []string {
	var bad []string
	for _, line := range lines {
		if !strings.Contains(line, symbol) || isNumberedTestListItem(line) {
			continue
		}
		recordsRetirement := strings.Contains(line, "06-F2") && strings.Contains(line, "1864") && containsAny(line, retirementMarkers)
		if !recordsRetirement {
			bad = append(bad, strings.TrimSpace(line)[:min(90, len(strings.TrimSpace(line)))])
		}
	}
	return bad
}

func TestC1922_001_LineClassifierRejectsPresentTenseAndAcceptsRetirementRecord(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		wantBad bool
	}{
		{"present tense stays", "- **`RecordRootTaskFailure`** stays as a second quarantine writer.", true},
		{"retirement without cycle", "- `RecordRootTaskFailure` was retired under 06-F2.", true},
		{"cycle without retirement word", "- `RecordRootTaskFailure` 06-F2 cycle 1864.", true},
		{"full record", "- `RecordRootTaskFailure` was retired under 06-F2 in cycle 1864.", false},
		{"no mention", "- Something else entirely.", false},
		{"numbered test list exempt", "49. `TestFacades_KeepEverySpelling` names `RecordRootTaskFailure`", false},
	}
	for _, c := range cases {
		got := len(linesViolatingRetirementRecord([]string{c.line})) > 0
		if got != c.wantBad {
			t.Errorf("%s: violation=%v want %v", c.name, got, c.wantBad)
		}
	}
}

func TestC1922_002_DecompositionDocRecordsRetirementOnEveryMention(t *testing.T) {
	bad := linesViolatingRetirementRecord(docLines(t, decompositionRel))
	if len(bad) > 0 {
		t.Errorf("RED: %s has %d mention(s) of %s without 06-F2 + cycle 1864 + a retirement word: %q", decompositionRel, len(bad), symbol, bad)
	}
}

func TestC1922_003_PackageDocRecordsRetirementOnEveryMention(t *testing.T) {
	bad := linesViolatingRetirementRecord(docLines(t, packageDocRel))
	if len(bad) > 0 {
		t.Errorf("RED: %s has %d mention(s) of %s without 06-F2 + cycle 1864 + a retirement word: %q", packageDocRel, len(bad), symbol, bad)
	}
}

func TestC1922_004_NoDocStillDescribesTheRetiredWriterAsPresent(t *testing.T) {
	for _, rel := range []string{decompositionRel, packageDocRel} {
		for n, line := range docLines(t, rel) {
			if !strings.Contains(line, symbol) {
				continue
			}
			if containsAny(line, presentTenseClaims) {
				t.Errorf("RED: %s:%d still describes %s as present: %.100s", rel, n+1, symbol, strings.TrimSpace(line))
			}
		}
	}
}

func TestC1922_005_DecompositionFollowUpNoLongerPlansToDeleteIt(t *testing.T) {
	for n, line := range docLines(t, decompositionRel) {
		if strings.Contains(line, "06-F2 `"+symbol+"` — delete") {
			t.Errorf("RED: %s:%d still lists 06-F2 as a pending follow-up (delete or fold)", decompositionRel, n+1)
		}
	}
}

func TestC1922_006_RetiredSymbolStaysAbsentFromProductionGo(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "grep", "-l", symbol, "--", "go/*.go")
	if err != nil && code != 1 {
		t.Fatalf("git grep failed: code=%d err=%v", code, err)
	}
	for _, f := range strings.Fields(out) {
		if strings.HasSuffix(f, "_test.go") || strings.HasPrefix(f, "go/acs/") {
			continue
		}
		t.Errorf("preservation broken: %s references %s", f, symbol)
	}
}

func TestC1922_007_ChangedDocsPassSteLintOnAddedLinesAndBothDocsChanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "HEAD", "--", decompositionRel, packageDocRel)
	if err != nil || code != 0 {
		t.Fatalf("git diff failed: code=%d err=%v", code, err)
	}
	for _, rel := range []string{decompositionRel, packageDocRel} {
		if !strings.Contains(changed, rel) {
			t.Errorf("RED: %s is not modified relative to HEAD", rel)
		}
	}
	bin := filepath.Join(root, "go", "bin", "evolve")
	stdout, stderr, lintCode, lintErr := acsassert.SubprocessOutput(bin, "docs", "ste-lint", "--strict", "--changed", "HEAD", "--added", "--project-root", root)
	if lintErr != nil || lintCode != 0 {
		t.Errorf("RED: ste-lint --strict on added lines exit=%d err=%v\n%s%s", lintCode, lintErr, stdout, stderr)
	}
}
