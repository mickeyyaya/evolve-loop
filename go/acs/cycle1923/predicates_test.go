//go:build acs

package cycle1923

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	decompositionRel = "docs/architecture/decomposition/06-inboxmover.md"
	maxSentenceWords = 25
)

type followUp struct {
	id      string
	anchors []string
}

var followUps = []followUp{
	{"06-F1", []string{"ship.Options", "cmd_ship.go:88-99", "postship.go:169", "newRootSignalCenter", "sandbox.write_subpaths", "signalcenter.sink_dropped", "cmd_cycle_ledger_roots_test.go:30"}},
	{"06-F2", []string{"RecordRootTaskFailure", "1864", "retired"}},
	{"06-F3", []string{"shaLandedOnMain", "review fold 2", "share"}},
	{"06-F4", []string{".evolve/runs/cycle-N", ".evolve/cycle-state.json", "internal/paths", "core.RunWorkspacePath", "cyclestate.State.CycleID"}},
	{"06-F5", []string{"five raw", "lifecycle.LegacyPrefix", "review fold 3", "logf"}},
	{"06-F6", []string{"LifecycleRecord", "orchestrator.go:429", ":443"}},
	{"06-F7", []string{"(*Mover).ApplyOutcome", "ClaimScope", "outcome.go:151"}},
	{"06-F8", []string{"processed|rejected|retry", "cmd_inbox_mover.go:104", "test 15"}},
	{"06-F9", []string{"dashboard/queue.go:15", "lifecycleDirs", "rejected", "quarantine"}},
	{"06-F10", []string{"postship.go:313-325", "shaLandedOnMain"}},
	{"06-F11", []string{"ClaimLaneScope", "Locate", "INBOX_CLAIM_NOT_FOUND", "test 50"}},
}

var reintroducedPresentTense = []string{
	"or fold onto",
	"stays as a second quarantine writer",
	"an operator who later wires it",
}

var codeSpan = regexp.MustCompile("`[^`]*`")
var sentenceEnd = regexp.MustCompile(`[.;!?](\s+|$)`)
var steFinding = regexp.MustCompile(`:(\d+) STE-`)

func readLines(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(decompositionRel)))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(raw), "\n")
}

func followUpBlocks(lines []string) map[string][]int {
	blocks := map[string][]int{}
	current := ""
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "- 06-F"):
			rest := strings.TrimPrefix(line, "- ")
			end := strings.IndexAny(rest, ": ")
			if end < 0 {
				end = len(rest)
			}
			current = rest[:end]
			blocks[current] = append(blocks[current], i)
		case current != "" && strings.TrimSpace(line) != "":
			blocks[current] = append(blocks[current], i)
		default:
			current = ""
		}
	}
	return blocks
}

func blockText(lines []string, idx []int) string {
	var parts []string
	for _, i := range idx {
		parts = append(parts, lines[i])
	}
	return strings.Join(parts, "\n")
}

func longSentences(text string) []string {
	var bad []string
	for _, line := range strings.Split(text, "\n") {
		flat := codeSpan.ReplaceAllString(line, "X")
		for _, s := range sentenceEnd.Split(flat, -1) {
			if n := len(strings.Fields(s)); n > maxSentenceWords {
				bad = append(bad, strings.TrimSpace(s))
			}
		}
	}
	return bad
}

func TestC1923_001_BlockExtractorGroupsContinuationLinesUnderTheirBullet(t *testing.T) {
	lines := []string{
		"Follow-ups, in order:",
		"- 06-F1: first.",
		"  - detail one.",
		"- 06-F2: second.",
		"",
		"unrelated paragraph.",
	}
	blocks := followUpBlocks(lines)
	if got := blockText(lines, blocks["06-F1"]); !strings.Contains(got, "detail one") || strings.Contains(got, "second") {
		t.Errorf("06-F1 block wrong: %q", got)
	}
	if got := blockText(lines, blocks["06-F2"]); strings.Contains(got, "unrelated") {
		t.Errorf("06-F2 block leaked past the blank line: %q", got)
	}
	if _, ok := blocks["06-F3"]; ok {
		t.Error("phantom block for 06-F3")
	}
}

func TestC1923_002_LongSentenceCounterFlagsOnlyOverLimit(t *testing.T) {
	short := "Thread the `Center` into twelve roots."
	long := strings.Repeat("word ", maxSentenceWords+1) + "end."
	if got := longSentences(short); len(got) != 0 {
		t.Errorf("short sentence flagged: %v", got)
	}
	if got := longSentences(short + " " + long); len(got) != 1 {
		t.Errorf("want exactly the long sentence flagged, got %v", got)
	}
}

func TestC1923_003_EveryFollowUpRestoresItsFileLineAnchors(t *testing.T) {
	lines := readLines(t)
	blocks := followUpBlocks(lines)
	for _, f := range followUps {
		idx, ok := blocks[f.id]
		if !ok {
			t.Errorf("%s: bullet missing", f.id)
			continue
		}
		text := blockText(lines, idx)
		for _, a := range f.anchors {
			if !strings.Contains(text, a) {
				t.Errorf("%s: anchor %q not restored under its bullet", f.id, a)
			}
		}
	}
}

func TestC1923_004_RetirementRecordOf06F2SurvivesTheRestoreAndDoesNotRegressToPresentTense(t *testing.T) {
	lines := readLines(t)
	blocks := followUpBlocks(lines)
	idx, ok := blocks["06-F2"]
	if !ok {
		t.Fatal("06-F2 bullet missing")
	}
	text := blockText(lines, idx)
	for _, w := range []string{"1864", "RecordRootTaskFailure"} {
		if !strings.Contains(text, w) {
			t.Errorf("06-F2 lost %q", w)
		}
	}
	whole := strings.Join(lines, "\n")
	for _, bad := range reintroducedPresentTense {
		if strings.Contains(whole, bad) {
			t.Errorf("present-tense claim about the retired writer is back: %q", bad)
		}
	}
}

func TestC1923_005_FollowUpBlockSentencesAreWithinTheSteLimit(t *testing.T) {
	lines := readLines(t)
	blocks := followUpBlocks(lines)
	if len(blocks) < len(followUps) {
		t.Fatalf("only %d follow-up blocks found, want %d", len(blocks), len(followUps))
	}
	for _, f := range followUps {
		if long := longSentences(blockText(lines, blocks[f.id])); len(long) > 0 {
			t.Errorf("%s: %d sentence(s) over %d words, first: %q", f.id, len(long), maxSentenceWords, long[0])
		}
	}
}

func TestC1923_006_SteLintReportsNothingInsideTheFollowUpBlock(t *testing.T) {
	root := acsassert.RepoRoot(t)
	lines := readLines(t)
	blocks := followUpBlocks(lines)
	if len(blocks) < len(followUps) {
		t.Fatalf("only %d follow-up blocks found, want %d", len(blocks), len(followUps))
	}
	inBlock := map[int]bool{}
	for _, idx := range blocks {
		for _, i := range idx {
			inBlock[i+1] = true
		}
	}
	out, errOut, code, _ := acsassert.SubprocessOutput(filepath.Join(root, "go", "bin", "evolve"),
		"docs", "ste-lint", "--strict", "--project-root", root, filepath.Join(root, filepath.FromSlash(decompositionRel)))
	if code > 1 || !strings.Contains(out+errOut, "ste-lint:") {
		t.Fatalf("ste-lint did not run: exit %d, output %q", code, out+errOut)
	}
	for _, m := range steFinding.FindAllStringSubmatch(out+errOut, -1) {
		n, _ := strconv.Atoi(m[1])
		if inBlock[n] {
			t.Errorf("ste-lint finding inside the follow-up block at line %d", n)
		}
	}
}
